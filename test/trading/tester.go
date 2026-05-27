// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"context"

	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
)

// TradeTester is a helper for testing trading engines and middlewares.
type TradeTester struct {
	prices      map[string]int
	middlewares []engine.Middleware
}

// NewTradeTester creates a new TradeTester.
func NewTradeTester() *TradeTester {
	return &TradeTester{
		prices:      make(map[string]int),
		middlewares: make([]engine.Middleware, 0),
	}
}

// WithPrices sets the base pricing data for the tester.
func (t *TradeTester) WithPrices(prices map[string]int) *TradeTester {
	t.prices = prices
	return t
}

// AddMiddleware registers a middleware to be executed during the trade process.
func (t *TradeTester) AddMiddleware(mw engine.Middleware) *TradeTester {
	t.middlewares = append(t.middlewares, mw)
	return t
}

// Run executes the trade offer through the middleware chain under a mocked pricing feed.
func (t *TradeTester) Run(ctx context.Context, offer *trading.TradeOffer) (*engine.Verdict, error) {
	eng := engine.New()

	// Inject standard price feed middleware
	eng.Use(func(next engine.Handler) engine.Handler {
		return func(c *engine.TradeContext) error {
			for sku, price := range t.prices {
				c.Set("price_"+sku, price)
			}
			return next(c)
		}
	})

	// Add all registered middlewares
	eng.Use(t.middlewares...)

	return eng.Process(ctx, offer)
}
