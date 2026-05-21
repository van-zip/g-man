// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rest

import (
	"errors"
	"fmt"
	"io"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// ProtobufDecoder decodes Protobuf data. It automatically detects if the
// source is JSON-encoded Protobuf or standard binary wire format.
var ProtobufDecoder Decoder = DecoderFunc(func(r io.Reader, target any) error {
	pm, ok := target.(proto.Message)
	if !ok {
		return errors.New("rest: target is not a proto.Message")
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	if len(data) > 0 && data[0] == '{' {
		return protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(data, pm)
	}

	return proto.Unmarshal(data, pm)
})

// RawDecoder returns the response body as a byte slice.
// It expects target to be a pointer to a byte slice (*[]byte).
var RawDecoder Decoder = DecoderFunc(func(r io.Reader, target any) error {
	ptr, ok := target.(*[]byte)
	if !ok {
		return fmt.Errorf("rest: RawDecoder requires *[]byte as output type, got %T", target)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	*ptr = data

	return nil
})

// AsProtobuf returns a RequestModifier that sets the decoder to ProtobufDecoder.
func AsProtobuf() RequestModifier { return WithDecoder(ProtobufDecoder) }

// AsRaw returns a RequestModifier that sets the decoder to RawDecoder.
func AsRaw() RequestModifier { return WithDecoder(RawDecoder) }
