// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package inventory allows retrieving user inventory using community requester.
package inventory

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/lemon4ksan/g-man/pkg/steam/api"
	"github.com/lemon4ksan/g-man/pkg/steam/community"
)

// GetUserInventoryContents recursively parses user inventory using
// community requester and returns the list of items and currencies
// with their total count.
func GetUserInventoryContents(
	ctx context.Context,
	c community.Requester,
	userID uint64,
	appID uint32,
	contextID int64,
	tradableOnly bool,
	language string,
) ([]CEconItem, []CEconItem, int, error) {
	if language == "" {
		language = "english"
	}

	var (
		inventory    []CEconItem
		currency     []CEconItem
		startAssetID string
		totalCount   int
	)

	pos := 1

	for {
		path := fmt.Sprintf("inventory/%d/%d/%d", userID, appID, contextID)

		req := struct {
			Language     string `url:"l"`
			Count        int    `url:"count"`
			StartAssetID string `url:"start_assetid,omitempty"`
		}{Language: language, Count: 1000, StartAssetID: startAssetID}

		resp, err := community.Get[inventoryResponse](ctx, c, path, req, api.WithHeader(
			"Referer", fmt.Sprintf("https://steamcommunity.com/profiles/%d/inventory", userID)),
		)
		if err != nil {
			return nil, nil, 0, err
		}

		if !resp.Success {
			return nil, nil, 0, fmt.Errorf("steam error: %s", resp.Error)
		}

		if resp.TotalCount == 0 || (len(resp.Assets) == 0) {
			return inventory, currency, resp.TotalCount, nil
		}

		descMap := make(map[string]*Description)
		for _, d := range resp.Descriptions {
			key := fmt.Sprintf("%s_%s", d.ClassID, d.InstanceID)
			descMap[key] = &d
		}

		for _, asset := range resp.Assets {
			key := fmt.Sprintf("%s_%s", asset.ClassID, asset.InstanceID)
			description := descMap[key]

			if tradableOnly && (description == nil || description.Tradable == 0) {
				continue
			}

			asset.Pos = pos
			pos++

			item := CEconItem{
				Asset:       asset,
				Description: description,
			}

			if asset.CurrencyID != "" {
				currency = append(currency, item)
			} else {
				inventory = append(inventory, item)
			}
		}

		totalCount = resp.TotalCount

		if !resp.MoreItems {
			break
		}

		startAssetID = resp.LastAssetID
	}

	return inventory, currency, totalCount, nil
}

var rxAppContextData = regexp.MustCompile(`(?s)var g_rgAppContextData\s*=\s*(.*?);`)

// GetUserInventoryContexts retrieves the application and context details for a user's inventory.
func GetUserInventoryContexts(
	ctx context.Context,
	c community.Requester,
	userID uint64,
) (map[string]*AppContext, error) {
	path := fmt.Sprintf("profiles/%d/inventory", userID)

	htmlBytes, err := community.GetHTML(ctx, c, path)
	if err != nil {
		return nil, fmt.Errorf("inventory: failed to fetch inventory page: %w", err)
	}

	if bytes.Contains(htmlBytes, []byte("This profile is private.")) {
		return nil, errors.New("inventory: profile is private")
	}

	if bytes.Contains(htmlBytes, []byte("The inventory is currently private.")) ||
		bytes.Contains(htmlBytes, []byte("inventory is currently private")) {
		return nil, errors.New("inventory: inventory is private")
	}

	match := rxAppContextData.FindSubmatch(htmlBytes)
	if len(match) != 2 {
		return nil, errors.New("inventory: malformed page (g_rgAppContextData not found)")
	}

	// In some cases, if the inventory is empty, Steam might return an empty JSON array `[]` instead of `{}`,
	// which fails to unmarshal into map[string]*AppContext. We check for `[]` first.
	cleanedJSON := bytes.TrimSpace(match[1])
	if bytes.Equal(cleanedJSON, []byte("[]")) {
		return make(map[string]*AppContext), nil
	}

	var data map[string]*AppContext
	if err := json.Unmarshal(cleanedJSON, &data); err != nil {
		return nil, fmt.Errorf("inventory: failed to parse context data JSON: %w", err)
	}

	return data, nil
}
