// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package api provides a specialized framework for interacting with Steam Web APIs.

It bridges the gap between raw network transport and domain-specific logic by
handling Steam's inconsistent response formats and parameter requirements.

# Key Components

  - [UnmarshalRegistry]: A thread-safe registry of decoders used to parse multiple response formats.
  - [ResponseFormat]: An enumeration defining supported formats (JSON, VDF, Protobuf, Binary VDF).
  - [HTTPTarget]: A basic implementation of the transport Target interface for HTTP-based calls.
  - [SteamAPIError]: A structured error container that captures raw API response failures.
  - [EResultError]: An error wrapper around Steam's internal EResult enum codes.

# Basic Usage Example

	package main

	import (
		"fmt"
		"github.com/lemon4ksan/g-man/pkg/steam/api"
	)

	type Summary struct {
		SteamID uint64 `json:"steamid"`
	}

	func main() {
		// Create a new unmarshal registry
		reg := api.NewUnmarshalRegistry()

		// Mock raw Steam WebAPI JSON response data (usually wrapped under "response")
		rawData := []byte(`{"response":{"steamid":76561197960265728}}`)

		var res Summary
		err := reg.Unmarshal(rawData, &res, api.FormatJSON)
		if err != nil {
			fmt.Println("Unmarshal failed:", err)
			return
		}

		fmt.Println("Parsed SteamID:", res.SteamID)
	}
*/
package api
