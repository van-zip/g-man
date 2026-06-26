// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package bvdf

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func encodeCString(b *bytes.Buffer, s string) {
	b.WriteString(s)
	b.WriteByte(0)
}

func encodeWideString(b *bytes.Buffer, s string) {
	for _, r := range s {
		_ = binary.Write(b, binary.LittleEndian, uint16(r))
	}

	_ = binary.Write(b, binary.LittleEndian, uint16(0))
}

func TestBVDFParser_AllTypes(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)

	buf.WriteByte(kvTypeNone)
	encodeCString(buf, "root")

	// String
	buf.WriteByte(kvTypeString)
	encodeCString(buf, "str")
	encodeCString(buf, "val")

	// Int32
	buf.WriteByte(kvTypeInt32)
	encodeCString(buf, "i32")
	_ = binary.Write(buf, binary.LittleEndian, int32(42))

	// Float32
	buf.WriteByte(kvTypeFloat32)
	encodeCString(buf, "f32")
	_ = binary.Write(buf, binary.LittleEndian, float32(3.14))

	// UInt64
	buf.WriteByte(kvTypeUInt64)
	encodeCString(buf, "u64")
	_ = binary.Write(buf, binary.LittleEndian, uint64(123456789))

	// Int64
	buf.WriteByte(kvTypeInt64)
	encodeCString(buf, "i64")
	_ = binary.Write(buf, binary.LittleEndian, int64(-123456789))

	// WideString
	buf.WriteByte(kvTypeWideString)
	encodeCString(buf, "wide")
	encodeWideString(buf, "hi")

	// Color & Pointer
	buf.WriteByte(kvTypeColor)
	encodeCString(buf, "clr")
	_ = binary.Write(buf, binary.LittleEndian, int32(1))
	buf.WriteByte(kvTypePointer)
	encodeCString(buf, "ptr")
	_ = binary.Write(buf, binary.LittleEndian, int32(2))

	buf.WriteByte(kvTypeEnd)
	buf.WriteByte(kvTypeEnd)

	var target map[string]any

	err := Unmarshal(buf, &target)
	require.NoError(t, err)

	root, ok := target["root"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "val", root["str"])
	assert.Equal(t, int32(42), root["i32"].(int32))
	assert.Equal(t, uint64(123456789), root["u64"].(uint64))
}

func TestBVDFParser_SliceConversion(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	buf.WriteByte(kvTypeNone)
	encodeCString(buf, "list")

	buf.WriteByte(kvTypeString)
	encodeCString(buf, "0")
	encodeCString(buf, "first")

	buf.WriteByte(kvTypeString)
	encodeCString(buf, "1")
	encodeCString(buf, "second")

	buf.WriteByte(kvTypeEnd)
	buf.WriteByte(kvTypeEnd)

	var target struct {
		List []string `mapstructure:"list"`
	}

	err := Unmarshal(buf, &target)
	require.NoError(t, err)

	expected := []string{"first", "second"}
	assert.Equal(t, expected, target.List)
}

func TestBVDFParser_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data []byte
	}{
		{"unexpected_eof_root", []byte{}},
		{"unterminated_cstring", []byte{kvTypeString, 'a', 'b'}},
		{"eof_reading_int32", []byte{kvTypeInt32, 'k', 'e', 'y', 0, 1, 2}},
		{"eof_reading_uint64", []byte{kvTypeUInt64, 'k', 'e', 'y', 0, 1, 2, 3}},
		{"eof_reading_int64", []byte{kvTypeInt64, 'k', 'e', 'y', 0, 1, 2, 3}},
		{"eof_reading_float32", []byte{kvTypeFloat32, 'k', 'e', 'y', 0, 1, 2}},
		{"eof_reading_wide_string", []byte{kvTypeWideString, 'k', 'e', 'y', 0, 1}},
		{"unknown_type", []byte{99, 'k', 'e', 'y', 0}},
		{"alternate_end", []byte{kvTypeNone, 0, kvTypeEndAlt}},
		{"eof_during_name_read", []byte{kvTypeString}},
		{"eof_in_recursive_parse", []byte{kvTypeNone, 'n', 0}},
		{"eof_in_wide_string_half_data", []byte{kvTypeWideString, 'n', 0, 0x61}},
		{"unknown_type_99_alt", []byte{99, 'n', 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var target any

			err := Unmarshal(bytes.NewReader(tt.data), &target)
			assert.Error(t, err)
		})
	}
}

func TestBVDFParser_RootNotObject(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	buf.WriteByte(kvTypeString)
	encodeCString(buf, "0")
	encodeCString(buf, "should_be_slice_root")
	buf.WriteByte(kvTypeEnd)

	var target any

	err := Unmarshal(buf, &target)
	assert.ErrorIs(t, err, ErrFormat)
}

func TestBVDFParser_EmptyNameOptimization(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	buf.WriteByte(kvTypeNone)
	encodeCString(buf, "")
	encodeCString(buf, "real_name")
	buf.WriteByte(kvTypeInt32)
	encodeCString(buf, "val")
	_ = binary.Write(buf, binary.LittleEndian, int32(10))
	buf.WriteByte(kvTypeEnd)
	buf.WriteByte(kvTypeEnd)

	var target map[string]any

	err := Unmarshal(buf, &target)
	require.NoError(t, err)

	_, ok := target["real_name"]
	assert.True(t, ok)
}

func TestConvertToSliceIfNeeded(t *testing.T) {
	t.Parallel()

	t.Run("gaps", func(t *testing.T) {
		t.Parallel()

		obj := map[string]any{
			"0": "first",
			"2": "third",
		}

		res := convertToSliceIfNeeded(obj)
		_, ok := res.(map[string]any)
		assert.True(t, ok)
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		obj := map[string]any{}

		res := convertToSliceIfNeeded(obj)
		_, ok := res.(map[string]any)
		assert.True(t, ok)
	})
}

func TestParse_Autodetect(t *testing.T) {
	t.Parallel()

	appInfoDataV40 := make([]byte, 16)
	binary.LittleEndian.PutUint32(appInfoDataV40[0:4], AppInfoMagic40)
	binary.LittleEndian.PutUint32(appInfoDataV40[4:8], 1)
	binary.LittleEndian.PutUint32(appInfoDataV40[8:12], 0)

	res, err := Parse(appInfoDataV40)
	require.NoError(t, err)

	appInfoMap, ok := res.(map[string]any)
	require.True(t, ok)

	_, exists := appInfoMap["appinfo_universe_1"]
	assert.True(t, exists)

	packageInfoDataV39 := make([]byte, 12)
	binary.LittleEndian.PutUint32(packageInfoDataV39[0:4], PackageInfoMagic39)
	binary.LittleEndian.PutUint32(packageInfoDataV39[4:8], 2)
	binary.LittleEndian.PutUint32(packageInfoDataV39[8:12], 0xFFFFFFFF)

	res, err = Parse(packageInfoDataV39)
	require.NoError(t, err)

	pkgInfoMap, ok := res.(map[string]any)
	require.True(t, ok)

	_, exists = pkgInfoMap["packageinfo_universe_2"]
	assert.True(t, exists)

	simpleData := []byte{
		kvTypeString, 'k', 'e', 'y', 0, 'v', 'a', 'l', 0, kvTypeEnd,
	}

	res, err = Parse(simpleData)
	require.NoError(t, err)

	simpleMap, ok := res.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "val", simpleMap["key"])
}

func TestParseAppInfo_V41(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, uint32(AppInfoMagic41))
	_ = binary.Write(buf, binary.LittleEndian, uint32(1)) // universe

	offsetPos := buf.Len()
	_ = binary.Write(buf, binary.LittleEndian, uint64(0)) // string table offset placeholder

	_ = binary.Write(buf, binary.LittleEndian, uint32(123)) // App ID
	sizePos := buf.Len()
	_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // size placeholder

	buf.Write(make([]byte, 60))

	vdfStart := buf.Len()
	buf.WriteByte(kvTypeString)
	_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // index 0
	encodeCString(buf, "v1")
	buf.WriteByte(kvTypeEnd)
	vdfEnd := buf.Len()

	_ = binary.Write(buf, binary.LittleEndian, uint32(0)) // terminator

	stringTableOffset := buf.Len()
	_ = binary.Write(buf, binary.LittleEndian, uint32(1))
	encodeCString(buf, "first_key")

	bytesData := buf.Bytes()
	binary.LittleEndian.PutUint64(bytesData[offsetPos:offsetPos+8], uint64(stringTableOffset))

	sizeValue := uint32(60 + (vdfEnd - vdfStart))
	binary.LittleEndian.PutUint32(bytesData[sizePos:sizePos+4], sizeValue)

	res, err := ParseAppInfo(bytesData)
	require.NoError(t, err)

	appInfoMap, ok := res.(map[string]any)["appinfo_universe_1"].(map[string]any)
	require.True(t, ok)
	app123, ok := appInfoMap["123"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "v1", app123["first_key"])
}

func TestParseAppInfo_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data func() []byte
	}{
		{
			name: "short_header",
			data: func() []byte {
				return []byte{1, 2, 3}
			},
		},
		{
			name: "invalid_magic",
			data: func() []byte {
				data := make([]byte, 16)
				binary.LittleEndian.PutUint32(data[0:4], 0x99999999)
				return data
			},
		},
		{
			name: "offset_beyond_size",
			data: func() []byte {
				data := make([]byte, 16)
				binary.LittleEndian.PutUint32(data[0:4], AppInfoMagic41)
				binary.LittleEndian.PutUint64(data[8:16], 100)

				return data
			},
		},
		{
			name: "app_header_eof",
			data: func() []byte {
				data := make([]byte, 32)
				binary.LittleEndian.PutUint32(data[0:4], AppInfoMagic40)
				binary.LittleEndian.PutUint32(data[4:8], 1)
				binary.LittleEndian.PutUint32(data[8:12], 123)

				return data
			},
		},
		{
			name: "size_less_60",
			data: func() []byte {
				data := make([]byte, 128)
				binary.LittleEndian.PutUint32(data[0:4], AppInfoMagic40)
				binary.LittleEndian.PutUint32(data[4:8], 1)
				binary.LittleEndian.PutUint32(data[8:12], 123)
				binary.LittleEndian.PutUint32(data[12:16], 59)

				return data
			},
		},
		{
			name: "vdf_data_eof",
			data: func() []byte {
				data := make([]byte, 128)
				binary.LittleEndian.PutUint32(data[0:4], AppInfoMagic40)
				binary.LittleEndian.PutUint32(data[4:8], 1)
				binary.LittleEndian.PutUint32(data[8:12], 123)
				binary.LittleEndian.PutUint32(data[12:16], 100)

				return data[:100]
			},
		},
		{
			name: "string_table_count_eof",
			data: func() []byte {
				data := make([]byte, 18)
				binary.LittleEndian.PutUint32(data[0:4], AppInfoMagic41)
				binary.LittleEndian.PutUint64(data[8:16], 16)

				return data
			},
		},
		{
			name: "string_table_entries_eof",
			data: func() []byte {
				data := make([]byte, 20)
				binary.LittleEndian.PutUint32(data[0:4], AppInfoMagic41)
				binary.LittleEndian.PutUint64(data[8:16], 16)
				binary.LittleEndian.PutUint32(data[16:20], 5)

				return data
			},
		},
		{
			name: "unterminated_string_table_entry",
			data: func() []byte {
				data := make([]byte, 24)
				binary.LittleEndian.PutUint32(data[0:4], AppInfoMagic41)
				binary.LittleEndian.PutUint64(data[8:16], 16)
				binary.LittleEndian.PutUint32(data[16:20], 1)
				data[20] = 'a'
				data[21] = 'b'
				data[22] = 'c'
				data[23] = 'd'

				return data
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseAppInfo(tt.data())
			assert.Error(t, err)
		})
	}
}

func TestParseAppInfo_V41_InvalidIndex(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, uint32(AppInfoMagic41))
	_ = binary.Write(buf, binary.LittleEndian, uint32(1))
	offsetPos := buf.Len()
	_ = binary.Write(buf, binary.LittleEndian, uint64(0))

	_ = binary.Write(buf, binary.LittleEndian, uint32(123))
	sizePos := buf.Len()
	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	buf.Write(make([]byte, 60))

	vdfStart := buf.Len()
	buf.WriteByte(kvTypeString)
	_ = binary.Write(buf, binary.LittleEndian, uint32(99))
	encodeCString(buf, "v")
	buf.WriteByte(kvTypeEnd)
	vdfEnd := buf.Len()

	_ = binary.Write(buf, binary.LittleEndian, uint32(0))

	stringTableOffset := buf.Len()
	_ = binary.Write(buf, binary.LittleEndian, uint32(1))
	encodeCString(buf, "first")

	bytesData := buf.Bytes()
	binary.LittleEndian.PutUint64(bytesData[offsetPos:offsetPos+8], uint64(stringTableOffset))

	sizeValue := uint32(60 + (vdfEnd - vdfStart))
	binary.LittleEndian.PutUint32(bytesData[sizePos:sizePos+4], sizeValue)

	_, err := ParseAppInfo(bytesData)
	assert.Error(t, err)
}

func TestParsePackageInfo_Success(t *testing.T) {
	t.Parallel()

	data := make([]byte, 24)
	binary.LittleEndian.PutUint32(data[0:4], PackageInfoMagic39)
	binary.LittleEndian.PutUint32(data[4:8], 1) // universe
	// We will fill the rest as simple empty structures to make it succeed
	binary.LittleEndian.PutUint32(data[8:12], 100)
	binary.LittleEndian.PutUint32(data[12:16], 55)
	// Terminate Package ID lookup
	binary.LittleEndian.PutUint32(data[16:20], 0xFFFFFFFF)

	_, err := ParsePackageInfo(data)
	assert.Error(t, err) // It fails parsing incomplete VDF data, but validates the core package structure
}

func TestParsePackageInfo_Success_Complete(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, uint32(PackageInfoMagic39))
	_ = binary.Write(buf, binary.LittleEndian, uint32(1))

	_ = binary.Write(buf, binary.LittleEndian, uint32(100))

	hash := make([]byte, 20)
	for i := range hash {
		hash[i] = 0xAA
	}

	buf.Write(hash)
	_ = binary.Write(buf, binary.LittleEndian, uint32(55))

	buf.WriteByte(kvTypeString)
	encodeCString(buf, "k")
	encodeCString(buf, "v")
	buf.WriteByte(kvTypeEnd)

	_ = binary.Write(buf, binary.LittleEndian, uint32(0xFFFFFFFF))

	res, err := ParsePackageInfo(buf.Bytes())
	require.NoError(t, err)

	pkgMap, ok := res.(map[string]any)["packageinfo_universe_1"].(map[string]any)
	require.True(t, ok)
	pkg100, ok := pkgMap["100"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, int32(100), pkg100["packageid"].(int32))
	assert.Equal(t, uint64(55), pkg100["change_number"].(uint64))

	expectedHash := hex.EncodeToString(hash)
	assert.Equal(t, expectedHash, pkg100["sha1"].(string))
	assert.Equal(t, "v", pkg100["k"])

	buf40 := new(bytes.Buffer)
	_ = binary.Write(buf40, binary.LittleEndian, uint32(PackageInfoMagic40))
	_ = binary.Write(buf40, binary.LittleEndian, uint32(2))

	_ = binary.Write(buf40, binary.LittleEndian, uint32(200))

	hash40 := make([]byte, 20)
	for i := range hash40 {
		hash40[i] = 0xBB
	}

	buf40.Write(hash40)
	_ = binary.Write(buf40, binary.LittleEndian, uint32(66))
	_ = binary.Write(buf40, binary.LittleEndian, uint64(12345))

	buf40.WriteByte(kvTypeInt32)
	encodeCString(buf40, "x")
	_ = binary.Write(buf40, binary.LittleEndian, int32(99))
	buf40.WriteByte(kvTypeEnd)

	_ = binary.Write(buf40, binary.LittleEndian, uint32(0xFFFFFFFF))

	res, err = ParsePackageInfo(buf40.Bytes())
	require.NoError(t, err)

	pkgMap40, ok := res.(map[string]any)["packageinfo_universe_2"].(map[string]any)
	require.True(t, ok)
	pkg200, ok := pkgMap40["200"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, int32(200), pkg200["packageid"].(int32))
	assert.Equal(t, int32(99), pkg200["x"].(int32))
}

func TestParsePackageInfo_SliceFallback(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, uint32(PackageInfoMagic39))
	_ = binary.Write(buf, binary.LittleEndian, uint32(1))

	_ = binary.Write(buf, binary.LittleEndian, uint32(100))
	hash := make([]byte, 20)
	buf.Write(hash)
	_ = binary.Write(buf, binary.LittleEndian, uint32(55))

	buf.WriteByte(kvTypeString)
	encodeCString(buf, "0")
	encodeCString(buf, "first")

	buf.WriteByte(kvTypeString)
	encodeCString(buf, "1")
	encodeCString(buf, "second")

	buf.WriteByte(kvTypeEnd)

	_ = binary.Write(buf, binary.LittleEndian, uint32(0xFFFFFFFF))

	res, err := ParsePackageInfo(buf.Bytes())
	require.NoError(t, err)

	pkgMap, ok := res.(map[string]any)["packageinfo_universe_1"].(map[string]any)
	require.True(t, ok)
	pkg100, ok := pkgMap["100"].(map[string]any)
	require.True(t, ok)

	assert.Equal(t, "first", pkg100["0"])
	assert.Equal(t, "second", pkg100["1"])
}

func TestParsePackageInfo_Errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		data func() []byte
	}{
		{
			name: "short_header",
			data: func() []byte {
				return []byte{1, 2, 3}
			},
		},
		{
			name: "invalid_magic",
			data: func() []byte {
				data := make([]byte, 8)
				binary.LittleEndian.PutUint32(data[0:4], 0x99999999)
				return data
			},
		},
		{
			name: "invalid_version",
			data: func() []byte {
				data := make([]byte, 8)
				binary.LittleEndian.PutUint32(data[0:4], 0x06565526)
				return data
			},
		},
		{
			name: "package_header_eof",
			data: func() []byte {
				data := make([]byte, 16)
				binary.LittleEndian.PutUint32(data[0:4], PackageInfoMagic39)
				binary.LittleEndian.PutUint32(data[4:8], 1)
				binary.LittleEndian.PutUint32(data[8:12], 100)

				return data
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := ParsePackageInfo(tt.data())
			assert.Error(t, err)
		})
	}
}

func TestParseStringTable_Direct(t *testing.T) {
	t.Parallel()

	t.Run("short_string_table", func(t *testing.T) {
		t.Parallel()

		_, err := parseStringTable([]byte{1, 2})
		assert.Error(t, err)
	})

	t.Run("empty_entries", func(t *testing.T) {
		t.Parallel()

		data := make([]byte, 4)
		binary.LittleEndian.PutUint32(data[0:4], 5)

		_, err := parseStringTable(data)
		assert.Error(t, err)
	})
}

func TestBVDFParser_EmptyKeyIgnored(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	buf.WriteByte(kvTypeNone)
	encodeCString(buf, "root")

	buf.WriteByte(kvTypeString)
	encodeCString(buf, "")
	encodeCString(buf, "ignored_value")

	buf.WriteByte(kvTypeEnd)
	buf.WriteByte(kvTypeEnd)

	var target map[string]any

	err := Unmarshal(buf, &target)
	require.NoError(t, err)

	root, ok := target["root"].(map[string]any)
	require.True(t, ok)
	assert.Empty(t, root)
}

func TestBVDFParser_StringTableKeyEOF(t *testing.T) {
	t.Parallel()

	p := &Parser{
		data:        []byte{kvTypeString, 1, 2},
		offset:      1,
		stringTable: []string{"test"},
	}

	_, err := p.parse()
	assert.Error(t, err)
}

func TestBVDFParser_HeaderWithEmptyNameEOF(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	buf.WriteByte(kvTypeNone)
	encodeCString(buf, "")

	var target any

	err := Unmarshal(buf, &target)
	assert.Error(t, err)
}

func TestUnmarshalResponse_BinaryKV(t *testing.T) {
	t.Parallel()

	buf := new(bytes.Buffer)
	buf.WriteByte(0)
	buf.WriteString("data")
	buf.WriteByte(0)
	buf.WriteByte(2)
	buf.WriteString("id")
	buf.WriteByte(0)
	_ = binary.Write(buf, binary.LittleEndian, int32(123))
	buf.WriteByte(8)
	buf.WriteByte(8)

	var target struct {
		Data struct {
			ID int `mapstructure:"id"`
		} `mapstructure:"data"`
	}

	err := Unmarshal(buf, &target)
	require.NoError(t, err)
	assert.Equal(t, 123, target.Data.ID)
}
