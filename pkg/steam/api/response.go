// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/andygrunwald/vdf"
	"github.com/mitchellh/mapstructure"

	"github.com/lemon4ksan/g-man/pkg/rest"
)

// ResponseFormat defines the expected encoding of the Steam API response.
type ResponseFormat int

const (
	// FormatUnknown is the default state.
	FormatUnknown ResponseFormat = iota
	// FormatRaw returns the response body as-is without parsing.
	FormatRaw
	// FormatProtobuf parses binary or JSON-encoded Protobuf messages.
	FormatProtobuf
	// FormatJSON parses standard JSON, automatically unwrapping the "response" field if present.
	FormatJSON
	// FormatVDF parses KeyValues/VDF text format.
	FormatVDF
	// FormatBinaryKV parses Binary KeyValues, which is a Valve-proprietary format
	FormatBinaryKV
)

// RegistryProvider defines the contract for components that maintain an UnmarshalRegistry.
type RegistryProvider interface {
	// Registry returns the underlying UnmarshalRegistry.
	Registry() *UnmarshalRegistry
}

// UnmarshalRegistry is a thread-safe registry of decoders.
//
// Create and initialize new instances of the registry using the [NewUnmarshalRegistry] constructor.
type UnmarshalRegistry struct {
	decoders map[ResponseFormat]rest.Decoder
}

// NewUnmarshalRegistry creates and initializes a new registry
// with standard decoders (JSON, Protobuf, VDF, BinaryKV, Raw).
func NewUnmarshalRegistry() *UnmarshalRegistry {
	r := &UnmarshalRegistry{
		decoders: make(map[ResponseFormat]rest.Decoder),
	}

	r.Register(FormatRaw, rest.RawDecoder)
	r.Register(FormatProtobuf, rest.ProtobufDecoder)
	r.Register(FormatJSON, SteamJSONDecoder)
	r.Register(FormatVDF, VDFDecoder)
	r.Register(FormatBinaryKV, BinaryVDFDecoder)

	return r
}

// Register registers a new decoding function for the specified format.
func (r *UnmarshalRegistry) Register(format ResponseFormat, d rest.Decoder) {
	r.decoders[format] = d
}

// Unmarshal searches the registry for a suitable decoder and runs it.
//
// If the data slice is empty, it returns nil immediately without executing any decoders.
// If no decoder is registered for the specified format, it returns [ErrFormat].
func (r *UnmarshalRegistry) Unmarshal(data []byte, target any, format ResponseFormat) error {
	if len(data) == 0 {
		return nil
	}

	fn, ok := r.decoders[format]
	if !ok {
		return fmt.Errorf("%w: unsupported or unregistered format %v", ErrFormat, format)
	}

	return fn.Decode(bytes.NewReader(data), target)
}

// Helper functions for backward compatibility and testing

// UnmarshalRaw implements the standard UnmarshalerFunc for the FormatRaw format.
func UnmarshalRaw(data []byte, target any) error {
	if err := rest.RawDecoder.Decode(bytes.NewReader(data), target); err != nil {
		return fmt.Errorf("%w: %w", ErrFormat, err)
	}

	return nil
}

// UnmarshalProtobuf decodes Protobuf data.
func UnmarshalProtobuf(data []byte, target any) error {
	if err := rest.ProtobufDecoder.Decode(bytes.NewReader(data), target); err != nil {
		return fmt.Errorf("%w: %w", ErrFormat, err)
	}

	return nil
}

// UnmarshalJSON decodes JSON data.
func UnmarshalJSON(data []byte, target any) error {
	if err := SteamJSONDecoder.Decode(bytes.NewReader(data), target); err != nil {
		return fmt.Errorf("%w: %w", ErrFormat, err)
	}

	return nil
}

// UnmarshalVDFText parses Valve Data Format (KeyValues) text.
func UnmarshalVDFText(data []byte, target any) error {
	if err := VDFDecoder.Decode(bytes.NewReader(data), target); err != nil {
		return fmt.Errorf("%w: %w", ErrFormat, err)
	}

	return nil
}

// UnmarshalBinaryKV parses a byte array in Binary KeyValues format.
func UnmarshalBinaryKV(data []byte, target any) error {
	if err := BinaryVDFDecoder.Decode(bytes.NewReader(data), target); err != nil {
		return fmt.Errorf("%w: %w", ErrFormat, err)
	}

	return nil
}

// UnmarshalBinaryKVOffset parses a byte array in Binary KeyValues format starting from a specific offset and updates the offset.
func UnmarshalBinaryKVOffset(data []byte, offset *int, target any) error {
	p := &bvdfParser{data: data, offset: *offset}

	res, err := p.parse()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrFormat, err)
	}

	*offset = p.offset

	parsed, ok := res.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: root of binary vdf is not an object", ErrFormat)
	}

	config := &mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           target,
		Squash:           true,
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}

	return decoder.Decode(parsed)
}

// rest.Decoder implementations

// SteamJSONDecoder wraps rest.JSONDecoder to automatically unwrap the "response" field
// common in Steam Web API.
var SteamJSONDecoder = rest.DecoderFunc(func(r io.Reader, target any) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(data, &wrapper); err == nil {
		if inner, ok := wrapper["response"]; ok {
			return json.Unmarshal(inner, target)
		}
	}

	return json.Unmarshal(data, target)
})

// VDFDecoder parses Valve Data Format (KeyValues) text.
var VDFDecoder = rest.DecoderFunc(func(r io.Reader, target any) error {
	p := vdf.NewParser(r)

	m, err := p.Parse()
	if err != nil {
		return err
	}

	config := &mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           target,
		Squash:           true,
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}

	if res, ok := m["response"].(map[string]any); ok {
		return decoder.Decode(res)
	}

	return decoder.Decode(m)
})

// BinaryVDFDecoder parses Valve Binary KeyValues format.
var BinaryVDFDecoder = rest.DecoderFunc(func(r io.Reader, target any) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	p := &bvdfParser{data: data, offset: 0}

	res, err := p.parse()
	if err != nil {
		return err
	}

	parsed, ok := res.(map[string]any)
	if !ok {
		return fmt.Errorf("%w: root of binary vdf is not an object", ErrFormat)
	}

	config := &mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		Result:           target,
		Squash:           true,
	}

	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return err
	}

	return decoder.Decode(parsed)
})
