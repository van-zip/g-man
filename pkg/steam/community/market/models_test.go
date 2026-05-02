// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package market

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAsset_UnmarshalJSON(t *testing.T) {
	jsonData := `{
		"appid": 730,
		"contextid": "2",
		"id": "123456789",
		"amount": "1",
		"tradable": 1,
		"commodity": 0,
		"marketable": "1",
		"name": "AK-47 | Redline"
	}`

	var asset Asset

	err := json.Unmarshal([]byte(jsonData), &asset)
	require.NoError(t, err)

	// Verify conversions worked
	assert.Equal(t, 730, asset.AppID)
	assert.Equal(t, int64(2), int64(asset.ContextID))
	assert.Equal(t, uint64(123456789), uint64(asset.ID))
	assert.True(t, bool(asset.Tradable))
	assert.False(t, bool(asset.Commodity))
	assert.True(t, bool(asset.Marketable))
	assert.Equal(t, "AK-47 | Redline", asset.Name)
}

func TestGraphPoints_UnmarshalJSON(t *testing.T) {
	t.Run("Valid Data", func(t *testing.T) {
		// Steam returns graph points as [price, volume, "description"]
		data := []byte(`[[10.5, 5, "5 sold at $10.50"], [11.0, 2, "2 sold at $11.00"]]`)

		var gp GraphPoints

		err := json.Unmarshal(data, &gp)
		require.NoError(t, err)
		require.Len(t, gp, 2)

		assert.Equal(t, 10.5, gp[0].Price)
		assert.Equal(t, int64(5), gp[0].Volume)
		assert.Equal(t, "5 sold at $10.50", gp[0].Description)
	})

	t.Run("Invalid Nested Length", func(t *testing.T) {
		// One point only has 2 elements instead of 3, should be skipped per logic
		data := []byte(`[[10.5, 5, "desc"], [11.0, 2]]`)

		var gp GraphPoints

		err := json.Unmarshal(data, &gp)
		require.NoError(t, err)
		assert.Len(t, gp, 2)
		assert.Equal(t, 0.0, gp[1].Price) // Second element was "continued" and remains default
	})

	t.Run("Malformed JSON", func(t *testing.T) {
		var gp GraphPoints

		err := json.Unmarshal([]byte(`{ "not": "an array" }`), &gp)
		require.Error(t, err)
	})
}

func TestPriceSample_UnmarshalJSON(t *testing.T) {
	t.Run("Valid Steam Format", func(t *testing.T) {
		// Layout: "Jan 02 2006 15:04:05 GMT-0700" (29 chars)
		// Slice: timeStr[:len-6]
		// Required input length: 29 + 6 = 35 chars
		timePart := "Jan 02 2026 15:04:05 GMT+0000" // 29 chars
		padding := " +0000"                         // 6 chars (including leading space)
		input := timePart + padding                 // Exactly 35 chars

		data := []byte(fmt.Sprintf(`["%s", 12.34, "500"]`, input))

		var ps PriceSample

		err := json.Unmarshal(data, &ps)
		require.NoError(t, err)

		assert.Equal(t, 12.34, ps.Price)
		assert.Equal(t, int64(500), ps.Volume)

		expected := time.Date(2026, time.January, 2, 15, 4, 5, 0, time.UTC)
		assert.True(t, ps.Timestamp.Equal(expected))
	})

	t.Run("Array Unmarshal Error", func(t *testing.T) {
		var ps PriceSample

		err := json.Unmarshal([]byte(`{"not": "an array"}`), &ps)
		require.Error(t, err)
	})

	t.Run("Timestamp Type Error", func(t *testing.T) {
		var ps PriceSample

		err := json.Unmarshal([]byte(`[123, 1.0, "1"]`), &ps)
		require.Error(t, err)
	})

	t.Run("Price Type Error", func(t *testing.T) {
		// We use a valid timestamp but a string for price where a float is expected
		timePart := "Jan 02 2026 15:04:05 GMT+0000 +0000"
		data := []byte(fmt.Sprintf(`["%s", "not-a-float", "1"]`, timePart))

		var ps PriceSample

		err := json.Unmarshal(data, &ps)
		require.Error(t, err)
	})

	t.Run("Volume Type Error", func(t *testing.T) {
		timePart := "Jan 02 2026 15:04:05 GMT+0000 +0000"
		data := []byte(fmt.Sprintf(`["%s", 1.0, 123]`, timePart)) // 123 is int, expected string

		var ps PriceSample

		err := json.Unmarshal(data, &ps)
		require.Error(t, err)
	})

	t.Run("Invalid Time Format", func(t *testing.T) {
		// Correct length but invalid date content
		badTime := "Invalid Date String Length 35!!!!"
		data := []byte(fmt.Sprintf(`["%s", 1.0, "1"]`, badTime))

		var ps PriceSample

		err := json.Unmarshal(data, &ps)
		require.Error(t, err)
	})
}

func TestBuyOrderResponse_Unmarshal(t *testing.T) {
	// Tests the struct tag `buy_orderid,string`
	data := []byte(`{"success": true, "buy_orderid": "987654321"}`)

	var resp CreateBuyOrderResponse

	err := json.Unmarshal(data, &resp)
	require.NoError(t, err)
	assert.True(t, resp.Success)
	assert.Equal(t, uint64(987654321), resp.BuyOrderID)
}

func TestItemOrdersHistogramResponse_Unmarshal(t *testing.T) {
	data := []byte(`{
		"success": 1,
		"sell_order_graph": [[100.0, 1, "1 unit"]]
	}`)

	var resp ItemOrdersHistogramResponse

	err := json.Unmarshal(data, &resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Success)
	require.Len(t, resp.SellOrderGraph, 1)
	assert.Equal(t, 100.0, resp.SellOrderGraph[0].Price)
}
