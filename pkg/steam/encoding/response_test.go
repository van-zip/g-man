// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package encoding

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

func UnmarshalResponse(data []byte, target any, format ResponseFormat) error {
	if len(data) == 0 {
		return nil
	}

	switch format {
	case FormatRaw:
		if ptr, ok := target.(*[]byte); ok {
			*ptr = append([]byte(nil), data...)
			return nil
		}

		return fmt.Errorf("%w: FormatRaw requires *[]byte as output type, got %T", ErrFormat, target)

	case FormatProtobuf:
		return ProtobufDecoder(bytes.NewReader(data), target)
	case FormatJSON:
		return SteamJSONDecoder(bytes.NewReader(data), target)
	case FormatVDF:
		return VDFDecoder(bytes.NewReader(data), target)
	case FormatBinaryVDF:
		return BinaryVDFDecoder(bytes.NewReader(data), target)
	default:
		return fmt.Errorf("%w: unsupported format %v", ErrFormat, format)
	}
}

func TestUnmarshalResponse(t *testing.T) {
	t.Run("Wrapped JSON", func(t *testing.T) {
		data := []byte(`{"response": {"name": "G-Man"}}`)
		target := make(map[string]string)

		err := UnmarshalResponse(data, &target, FormatJSON)
		if err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if target["name"] != "G-Man" {
			t.Errorf("expected G-Man, got %s", target["name"])
		}
	})

	t.Run("Direct JSON", func(t *testing.T) {
		data := []byte(`{"name": "Gordon"}`)
		target := make(map[string]string)

		err := UnmarshalResponse(data, &target, FormatJSON)
		if err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if target["name"] != "Gordon" {
			t.Errorf("expected Gordon, got %s", target["name"])
		}
	})

	t.Run("Protobuf", func(t *testing.T) {
		msg := &emptypb.Empty{}
		data, _ := proto.Marshal(msg)

		target := &emptypb.Empty{}

		err := UnmarshalResponse(data, &target, FormatProtobuf)
		if err != nil {
			t.Fatalf("unmarshal protobuf failed: %v", err)
		}
	})

	t.Run("VDF Text", func(t *testing.T) {
		data := []byte(`"Player" { "Health" "100" }`)

		var target struct {
			Player struct {
				Health string `mapstructure:"Health"`
			} `mapstructure:"Player"`
		}

		err := UnmarshalResponse(data, &target, FormatVDF)
		if err != nil {
			t.Fatalf("unmarshal VDF failed: %v", err)
		}

		if target.Player.Health != "100" {
			t.Errorf("expected 100, got %s", target.Player.Health)
		}
	})

	t.Run("FormatRaw", func(t *testing.T) {
		data := []byte("raw_binary_data")

		var target []byte

		err := UnmarshalResponse(data, &target, FormatRaw)
		if err != nil {
			t.Fatal(err)
		}

		if string(target) != "raw_binary_data" {
			t.Errorf("expected raw_binary_data, got %s", string(target))
		}

		// Test error when target is not *[]byte
		var wrongTarget string

		err = UnmarshalResponse(data, &wrongTarget, FormatRaw)
		if err == nil {
			t.Error("expected error for non-slice target in FormatRaw")
		}
	})

	t.Run("Protobuf JSON Detection", func(t *testing.T) {
		data := []byte(`{}`)
		target := &emptypb.Empty{}

		err := UnmarshalResponse(data, target, FormatProtobuf)
		if err != nil {
			t.Fatalf("failed to unmarshal JSON-encoded protobuf: %v", err)
		}
	})

	t.Run("VDF Wrapped", func(t *testing.T) {
		data := []byte(`"response" { "success" "1" }`)

		var target struct {
			Success string `mapstructure:"success"`
		}

		err := UnmarshalResponse(data, &target, FormatVDF)
		if err != nil {
			t.Fatal(err)
		}

		if target.Success != "1" {
			t.Error("failed to unwrap response in VDF")
		}
	})

	t.Run("Unsupported Format", func(t *testing.T) {
		err := UnmarshalResponse([]byte("data"), nil, FormatUnknown)
		if err == nil {
			t.Error("expected error for unknown format")
		}
	})
}

func TestEmptyResponse(t *testing.T) {
	target := make(map[string]any)

	cases := [][]byte{
		nil,
		{},
		[]byte(`{"response": {}}`),
		[]byte(`{"response": null}`),
	}

	for _, tc := range cases {
		err := UnmarshalResponse(tc, &target, FormatJSON)
		if err != nil {
			t.Errorf("expected no error for %v, got %v", tc, err)
		}
	}
}

func TestUnmarshalProtobuf_WrongType(t *testing.T) {
	err := ProtobufDecoder(bytes.NewReader([]byte("{}")), "not a proto message")
	if err == nil {
		t.Error("expected error for non-proto target")
	}
}

func TestUnmarshalVDFText_Invalid(t *testing.T) {
	err := VDFDecoder(bytes.NewReader([]byte("invalid vdf {")), &struct{}{})
	if err == nil {
		t.Error("expected error for invalid VDF text")
	}

	err = VDFDecoder(bytes.NewReader([]byte(`"Player" { "Health" "100" }`)), struct{}{})
	if err == nil {
		t.Error("expected error for invalid VDF text")
	}
}

func TestUnmarshalResponse_BinaryKV(t *testing.T) {
	buf := new(bytes.Buffer)
	buf.WriteByte(kvTypeNone)
	encodeCString(buf, "data")
	buf.WriteByte(kvTypeInt32)
	encodeCString(buf, "id")
	binary.Write(buf, binary.LittleEndian, int32(123))
	buf.WriteByte(kvTypeEnd)
	buf.WriteByte(kvTypeEnd)

	var target struct {
		Data struct {
			ID int `mapstructure:"id"`
		} `mapstructure:"data"`
	}

	err := UnmarshalResponse(buf.Bytes(), &target, FormatBinaryVDF)
	if err != nil {
		t.Fatalf("BinaryKV unmarshal failed: %v", err)
	}

	if target.Data.ID != 123 {
		t.Errorf("expected 123, got %d", target.Data.ID)
	}
}

func TestConvertToSliceIfNeeded_NotAllNumeric(t *testing.T) {
	obj := map[string]any{
		"0": "a",
		"2": "b",
	}

	res := convertToSliceIfNeeded(obj)
	if _, ok := res.(map[string]any); !ok {
		t.Error("expected map, got slice (gaps in indices should prevent slice conversion)")
	}
}

func TestReadWideString_ZeroTerminator(t *testing.T) {
	p := &bvdfParser{
		data: []byte{0x61, 0x00, 0x00, 0x00}, // 'a' + null terminator
	}

	str, err := p.readWideString()
	if err != nil || str != "a" {
		t.Errorf("expected 'a', got %s (err: %v)", str, err)
	}
}

func TestUnmarshalJSON_EdgeCases(t *testing.T) {
	t.Run("Invalid JSON", func(t *testing.T) {
		var target map[string]any

		err := SteamJSONDecoder(bytes.NewReader([]byte(`{invalid}`)), &target)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("Response field is not an object", func(t *testing.T) {
		data := []byte(`{"response": 123}`)

		var target int

		err := SteamJSONDecoder(bytes.NewReader(data), &target)
		if err != nil {
			t.Fatalf("should handle non-object response: %v", err)
		}

		if target != 123 {
			t.Errorf("expected 123, got %v", target)
		}
	})
}

func TestUnmarshalBinaryKV_Errors(t *testing.T) {
	t.Run("Invalid Header/Empty", func(t *testing.T) {
		err := BinaryVDFDecoder(bytes.NewReader([]byte{0xFF}), &struct{}{})
		if err == nil {
			t.Error("expected error for invalid binary KV data")
		}
	})

	t.Run("Not an object root", func(t *testing.T) {
		err := BinaryVDFDecoder(bytes.NewReader([]byte{kvTypeInt32}), &struct{}{})
		if err == nil {
			t.Error("expected error for malformed binary KV")
		}
	})

	t.Run("Not a pointer target", func(t *testing.T) {
		buf := new(bytes.Buffer)

		buf.WriteByte(kvTypeNone)
		encodeCString(buf, "root")

		buf.WriteByte(kvTypeString)
		encodeCString(buf, "str")
		encodeCString(buf, "val")

		buf.WriteByte(kvTypeEnd)
		buf.WriteByte(kvTypeEnd)

		err := BinaryVDFDecoder(buf, struct{}{})
		if err == nil {
			t.Error("expected error for non pointer target")
		}
	})
}

func TestBVDFParser_SpecificTypes(t *testing.T) {
	t.Run("Mapstructure Decode Error", func(t *testing.T) {
		buf := new(bytes.Buffer)
		buf.WriteByte(kvTypeNone)
		encodeCString(buf, "root")
		buf.WriteByte(kvTypeInt32)
		encodeCString(buf, "key")
		binary.Write(buf, binary.LittleEndian, int32(123))
		buf.WriteByte(kvTypeEnd)
		buf.WriteByte(kvTypeEnd)

		var target string
		if err := BinaryVDFDecoder(buf, &target); err == nil {
			t.Error("expected mapstructure error")
		}
	})
}
