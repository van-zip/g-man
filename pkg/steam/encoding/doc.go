// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package encoding provides decoders and request modifiers specifically tailored for Steam API responses.
// It automatically handles typical Steam Web API patterns, such as unwrapping the outer "response" JSON object,
// detecting Protobuf payload encodings, and parsing Valve-specific text and binary VDF formats.
//
// The package exposes [aoni.DecoderFunc] implementations like [SteamJSONDecoder] and [ProtobufDecoder],
// alongside [aoni.RequestModifier] factories like [AsJSON] to configure client requests.
//
// Basic usage example:
//
//	package main
//
//	import (
//		"context"
//		"fmt"
//
//		"github.com/lemon4ksan/aoni"
//		"github.com/lemon4ksan/g-man/pkg/steam/encoding"
//	)
//
//	type SteamApp struct {
//		AppID uint32 `json:"appid"`
//	}
//
//	func main() {
//		client := aoni.NewClient(nil)
//		var app SteamApp
//		// Automatically decode response and unwrap the "response" wrapper using AsJSON
//		_, err := client.Request(context.Background(), "GET", "api-endpoint", encoding.AsJSON())
//		if err != nil {
//			panic(err)
//		}
//		fmt.Println("Decoded AppID:", app.AppID)
//	}
package encoding
