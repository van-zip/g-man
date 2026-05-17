// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package stock

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/tf2/bptf"
	"github.com/lemon4ksan/g-man/pkg/tf2/currency"
	"github.com/lemon4ksan/g-man/pkg/tf2/trading"
)

type redirectTripper struct {
	targetURL string
}

func (rt *redirectTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	target, _ := url.Parse(rt.targetURL)
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host

	return http.DefaultClient.Do(req)
}

func TestStock_Undercutting(t *testing.T) {
	// 1. Create a temporary config file
	cfgFile := "test_trading_config.json"
	defer os.Remove(cfgFile)

	initialConfig := trading.Config{
		GlobalMaxStock:              3000,
		DefaultMaxStock:             5,
		ExcludedSteamIDs:            []string{"excluded_competitor_steam_id"},
		ExcludedListingDescriptions: []string{"exorcism", "chromatic"},
		PriceSwingLimits: trading.PriceSwingLimits{
			MaxBuyIncrease:  0.10, // 10%
			MaxSellDecrease: 0.10, // 10%
		},
		Items: map[string]trading.ItemConfig{
			"5021;6": {
				SKU:          "5021;6",
				Name:         "Mann Co. Supply Crate Key",
				MaxStock:     100,
				EnableBuy:    true,
				EnableSell:   true,
				MinBuyPrice:  currency.Currency{Keys: 0, Metal: 30.0},
				MaxBuyPrice:  currency.Currency{Keys: 0, Metal: 80.0},
				MinSellPrice: currency.Currency{Keys: 0, Metal: 35.0},
				MaxSellPrice: currency.Currency{Keys: 0, Metal: 85.0},
			},
		},
	}
	data, err := json.Marshal(initialConfig)
	require.NoError(t, err)
	err = os.WriteFile(cfgFile, data, 0o644)
	require.NoError(t, err)

	cfgMgr, err := trading.NewConfigManager(cfgFile)
	require.NoError(t, err)

	// Variables to dynamically control mock competitor responses
	var (
		competitorSellPrice = 57.0
		competitorBuyPrice  = 53.0
	)

	// 2. Set up a mock http server to simulate backpack.tf classified snapshot response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/classifieds/listings/snapshot", r.URL.Path)
		sku := r.URL.Query().Get("sku")
		appid := r.URL.Query().Get("appid")

		assert.Equal(t, "5021;6", sku)
		assert.Equal(t, "440", appid)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		resp := struct {
			Listings []bptf.ListingResponse `json:"listings"`
		}{
			Listings: []bptf.ListingResponse{
				{
					ID:      "comp1",
					SteamID: "competitor_steam_id_1",
					Intent:  "sell",
					Currencies: map[string]float64{
						"metal": competitorSellPrice,
					},
				},
				{
					ID:      "comp_excluded_steam",
					SteamID: "excluded_competitor_steam_id",
					Intent:  "sell",
					Currencies: map[string]float64{
						"metal": 51.0,
					},
				},
				{
					ID:      "comp_excluded_desc",
					SteamID: "competitor_steam_id_2",
					Intent:  "sell",
					Currencies: map[string]float64{
						"metal": 52.0,
					},
					Details: "Selling spelled items with exorcism!",
				},
				{
					ID:      "comp3",
					SteamID: "competitor_steam_id_3",
					Intent:  "buy",
					Currencies: map[string]float64{
						"metal": competitorBuyPrice,
					},
				},
				{
					ID:      "comp_excluded_buy_steam",
					SteamID: "excluded_competitor_steam_id",
					Intent:  "buy",
					Currencies: map[string]float64{
						"metal": 59.0,
					},
				},
				{
					ID:      "comp_excluded_buy_desc",
					SteamID: "competitor_steam_id_4",
					Intent:  "buy",
					Currencies: map[string]float64{
						"metal": 58.0,
					},
					Details: "Looking for chromatic paint spells",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 3. Initialize dependencies with the custom redirect client
	redirectClient := &http.Client{
		Transport: &redirectTripper{targetURL: server.URL},
	}

	bptfClient := bptf.New(redirectClient, "api-key", "token")
	listingMgr := bptf.NewListingManager(bptfClient, nil, log.New(log.DefaultConfig(log.ErrorLevel)))

	// 4. Construct behavior
	s := &Stock{
		listingMgr: listingMgr,
		cfgMgr:     cfgMgr,
		logger:     log.New(log.DefaultConfig(log.ErrorLevel)),
	}

	keyPriceRef := 60.0 // 1 key = 60.0 ref

	t.Run("Sell Undercutting", func(t *testing.T) {
		competitorSellPrice = 57.0
		// Lowest competitor sell price is 57.0 ref (513 scrap)
		// We undercut by 1 scrap -> 512 scrap (56.88 ref)
		// 512 scrap is between MinSellPrice (55.0 ref / 495 scrap) and MaxSellPrice (65.0 ref / 585 scrap)
		targetPriceScrap := currency.ToScrap(60.0) // priceDB sell target (e.g. 60.0 ref)

		finalPriceScrap, err := s.getUndercutPrice(
			context.Background(),
			"5021;6",
			"sell",
			targetPriceScrap,
			keyPriceRef,
		)
		require.NoError(t, err)

		expectedPriceScrap := currency.ToScrap(57.0) - 1
		assert.Equal(t, expectedPriceScrap, finalPriceScrap)
	})

	t.Run("Buy Overbidding", func(t *testing.T) {
		competitorBuyPrice = 53.0
		// Highest competitor buy price is 53.0 ref (477 scrap)
		// We overbid by 1 scrap -> 478 scrap (53.11 ref)
		// 478 scrap is between MinBuyPrice (50.0 ref / 450 scrap) and MaxBuyPrice (60.0 ref / 540 scrap)
		targetPriceScrap := currency.ToScrap(50.0) // priceDB buy target (e.g. 50.0 ref)

		finalPriceScrap, err := s.getUndercutPrice(context.Background(), "5021;6", "buy", targetPriceScrap, keyPriceRef)
		require.NoError(t, err)

		expectedPriceScrap := currency.ToScrap(53.0) + 1
		assert.Equal(t, expectedPriceScrap, finalPriceScrap)
	})

	t.Run("Undercut bound checking (floor)", func(t *testing.T) {
		// Set a competitor price below our MinSellPrice (e.g. 34.0 ref)
		// MinSellPrice is 35.0 ref (315 scrap).
		// Undercut should cap at 35.0 ref (315 scrap).
		competitorSellPrice = 34.0

		targetPriceScrap := currency.ToScrap(60.0)
		finalPriceScrap, err := s.getUndercutPrice(
			context.Background(),
			"5021;6",
			"sell",
			targetPriceScrap,
			keyPriceRef,
		)
		require.NoError(t, err)
		assert.Equal(t, currency.ToScrap(35.0), finalPriceScrap)
	})

	t.Run("Overbid bound checking (ceiling)", func(t *testing.T) {
		// Set a competitor price above our MaxBuyPrice (e.g. 81.0 ref)
		// MaxBuyPrice is 80.0 ref (720 scrap).
		// Overbid should cap at 80.0 ref (720 scrap).
		competitorBuyPrice = 81.0

		targetPriceScrap := currency.ToScrap(50.0)
		finalPriceScrap, err := s.getUndercutPrice(context.Background(), "5021;6", "buy", targetPriceScrap, keyPriceRef)
		require.NoError(t, err)
		assert.Equal(t, currency.ToScrap(80.0), finalPriceScrap)
	})

	t.Run("Price swing limits (sell decrease)", func(t *testing.T) {
		// Set a very low competitor sell price (e.g. 50.0 ref)
		competitorSellPrice = 50.0

		// Add an existing sell listing with price 60.0 ref (540 scrap)
		existingSell := &bptf.ListingResponse{
			ID:      "our_sell_listing",
			SteamID: "our_steam_id",
			Intent:  "sell",
			Details: "5021;6",
			Currencies: map[string]float64{
				"metal": 60.0,
			},
		}
		s.listingMgr.AddMockListing(existingSell)

		// MaxSellDecrease in config is 10% (0.10).
		// Max allowed sell decrease is 6.0 ref (54 scrap).
		// Capped price should be 60.0 - 6.0 = 54.0 ref (486 scrap).
		targetPriceScrap := currency.ToScrap(60.0)
		finalPriceScrap, err := s.getUndercutPrice(
			context.Background(),
			"5021;6",
			"sell",
			targetPriceScrap,
			keyPriceRef,
		)
		require.NoError(t, err)
		assert.Equal(t, currency.ToScrap(54.0), finalPriceScrap)
	})

	t.Run("Price swing limits (buy increase)", func(t *testing.T) {
		// Set a very high competitor buy price (e.g. 58.0 ref)
		competitorBuyPrice = 58.0

		// Add an existing buy listing with price 50.0 ref (450 scrap)
		existingBuy := &bptf.ListingResponse{
			ID:      "our_buy_listing",
			SteamID: "our_steam_id",
			Intent:  "buy",
			Details: "5021;6",
			Currencies: map[string]float64{
				"metal": 50.0,
			},
		}
		s.listingMgr.AddMockListing(existingBuy)

		// MaxBuyIncrease in config is 10% (0.10).
		// Max allowed buy increase is 5.0 ref (45 scrap).
		// Capped price should be 50.0 + 5.0 = 55.0 ref (495 scrap).
		targetPriceScrap := currency.ToScrap(50.0)
		finalPriceScrap, err := s.getUndercutPrice(
			context.Background(),
			"5021;6",
			"buy",
			targetPriceScrap,
			keyPriceRef,
		)
		require.NoError(t, err)
		assert.Equal(t, currency.ToScrap(55.0), finalPriceScrap)
	})
}
