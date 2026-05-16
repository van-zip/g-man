// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stock

import (
	"context"
	"fmt"
	"time"

	"github.com/lemon4ksan/g-man/pkg/behavior"
	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/tf2/backpack"
	"github.com/lemon4ksan/g-man/pkg/tf2/bptf"
	"github.com/lemon4ksan/g-man/pkg/tf2/pricedb"
	"github.com/lemon4ksan/g-man/pkg/tf2/trading"
)

// BehaviorName is the name of the stock control behavior.
const BehaviorName = "stock_control"

// WithBehavior returns an option that registers the stock control behavior with the orchestrator.
func WithBehavior(
	bp *backpack.Backpack,
	mgr *bptf.ListingManager,
	priceMgr *pricedb.Manager,
	cfg trading.StockConfig,
) behavior.Option {
	return func(o *behavior.Orchestrator) {
		o.Register(New(bp, mgr, priceMgr, o.Logger(), cfg))
	}
}

// Stock manages listings based on current stock levels and limits.
type Stock struct {
	bp         *backpack.Backpack
	listingMgr *bptf.ListingManager
	priceMgr   *pricedb.Manager
	logger     log.Logger
	config     trading.StockConfig
	interval   time.Duration
}

// New creates a new stock management behavior.
func New(
	bp *backpack.Backpack,
	mgr *bptf.ListingManager,
	priceMgr *pricedb.Manager,
	logger log.Logger,
	cfg trading.StockConfig,
) *Stock {
	return &Stock{
		bp:         bp,
		listingMgr: mgr,
		priceMgr:   priceMgr,
		logger:     logger.With(log.Module(BehaviorName)),
		config:     cfg,
		interval:   5 * time.Minute,
	}
}

// Name returns the unique name of the behavior.
func (s *Stock) Name() string {
	return BehaviorName
}

// Run starts the automated stock balancing loop.
func (s *Stock) Run(ctx context.Context) error {
	s.logger.Info("Stock Control behavior started", log.Duration("interval", s.interval))

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Initial check
	if err := s.rebalance(ctx); err != nil {
		s.logger.Error("Initial rebalance failed", log.Err(err))
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := s.rebalance(ctx); err != nil {
				s.logger.Error("Rebalance failed", log.Err(err))
			}
		}
	}
}

func (s *Stock) rebalance(ctx context.Context) error {
	s.logger.Debug("Rebalancing stock listings...")

	if err := s.listingMgr.Sync(ctx); err != nil {
		return fmt.Errorf("failed to sync listings: %w", err)
	}

	watchedSKUs := s.priceMgr.GetWatchedSKUs()

	for _, sku := range watchedSKUs {
		price, ok := s.priceMgr.GetPrice(sku)
		if !ok || price.Buy.IsZero() || price.Sell.IsZero() {
			continue
		}

		currentStock := s.bp.GetStock(sku)
		maxStock := s.getMaxStock(sku)

		// Buy listings
		if currentStock < maxStock {
			if err := s.ensureListing(ctx, sku, price, "buy"); err != nil {
				s.logger.Error("Failed to update buy listing", log.String("sku", sku), log.Err(err))
			}
		} else {
			s.removeListing(ctx, sku, "buy")
		}

		// Sell listings
		if currentStock > 0 {
			if err := s.ensureListing(ctx, sku, price, "sell"); err != nil {
				s.logger.Error("Failed to update sell listing", log.String("sku", sku), log.Err(err))
			}
		} else {
			s.removeListing(ctx, sku, "sell")
		}
	}

	return nil
}

func (s *Stock) ensureListing(ctx context.Context, sku string, price *pricedb.Price, intent string) error {
	existing := s.listingMgr.FindListingBySKU(sku, intent)

	targetPrice := price.Buy
	if intent == "sell" {
		targetPrice = price.Sell
	}

	if existing != nil {
		if s.isPriceSame(existing.Currencies, targetPrice) {
			return nil
		}

		if err := s.listingMgr.Delete(ctx, existing.ID); err != nil {
			return err
		}
	}

	currencies := map[string]float64{
		"metal": targetPrice.Metal,
	}
	if targetPrice.Keys > 0 {
		currencies["keys"] = float64(targetPrice.Keys)
	}

	_, err := s.listingMgr.Upsert(ctx, bptf.ListingResolvable{
		Item:       sku,
		Intent:     intent,
		Currencies: currencies,
		Details:    "SKU: " + sku,
	})

	return err
}

func (s *Stock) removeListing(ctx context.Context, sku, intent string) {
	if existing := s.listingMgr.FindListingBySKU(sku, intent); existing != nil {
		if err := s.listingMgr.Delete(ctx, existing.ID); err != nil {
			s.logger.Error("Failed to remove listing", log.String("sku", sku), log.Err(err))
		}
	}
}

func (s *Stock) isPriceSame(current map[string]float64, target pricedb.Currencies) bool {
	return current["keys"] == float64(target.Keys) && current["metal"] == target.Metal
}

func (s *Stock) getMaxStock(sku string) int {
	if max, ok := s.config.MaxPerSKU[sku]; ok {
		return max
	}

	return s.config.DefaultMax
}
