// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package protocol_test

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol/enums"
)

type faultyIO struct{}

func (f *faultyIO) Read(p []byte) (n int, err error)  { return 0, io.ErrUnexpectedEOF }
func (f *faultyIO) Write(p []byte) (n int, err error) { return 0, io.ErrShortWrite }

type infiniteZeros struct{}

func (infiniteZeros) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}

	return len(p), nil
}

func TestParsePacket(t *testing.T) {
	t.Run("ProtoPacket", func(t *testing.T) {
		hdr := protocol.NewMsgHdrProtoBuf(enums.EMsg(100), 1, 1)
		buf := new(bytes.Buffer)
		hdr.SerializeTo(buf)
		buf.WriteString("payload")

		pkt, err := protocol.ParsePacket(buf)
		require.NoError(t, err)
		assert.True(t, pkt.IsProto)
		assert.Equal(t, enums.EMsg(100), pkt.EMsg)
		assert.Equal(t, []byte("payload"), pkt.Payload)
	})

	t.Run("EncryptHandshakePacket", func(t *testing.T) {
		// Test specifically the EMsg branch for standard headers
		hdr := protocol.NewMsgHdr(enums.EMsg_ChannelEncryptRequest, 0)
		buf := new(bytes.Buffer)
		hdr.SerializeTo(buf)

		pkt, err := protocol.ParsePacket(buf)
		require.NoError(t, err)
		assert.False(t, pkt.IsProto)
		_, ok := pkt.Header.(*protocol.MsgHdr)
		assert.True(t, ok)
	})

	t.Run("ExtendedPacket", func(t *testing.T) {
		hdr := protocol.NewMsgHdrExtended(enums.EMsg(200), 1, 1)
		buf := new(bytes.Buffer)
		hdr.SerializeTo(buf)

		pkt, err := protocol.ParsePacket(buf)
		require.NoError(t, err)

		_, ok := pkt.Header.(*protocol.MsgHdrExtended)
		assert.True(t, ok)
	})

	t.Run("PayloadTooLarge", func(t *testing.T) {
		hdr := protocol.NewMsgHdr(enums.EMsg_ChannelEncryptRequest, 0)
		buf := new(bytes.Buffer)
		hdr.SerializeTo(buf)

		reader := io.MultiReader(buf, io.LimitReader(infiniteZeros{}, protocol.MaxPayloadSize+1))
		_, err := protocol.ParsePacket(reader)
		assert.ErrorIs(t, err, protocol.ErrPayloadTooLarge)
	})
}

func TestPacket_Getters(t *testing.T) {
	// Case: Header is nil (Coverage for p.Header != nil checks)
	pkt := &protocol.Packet{Header: nil}
	assert.Equal(t, protocol.NoJob, pkt.GetSourceJobID())
	assert.Equal(t, protocol.NoJob, pkt.GetTargetJobID())
	assert.Equal(t, uint64(0), pkt.GetSteamID())
	assert.Equal(t, int32(0), pkt.GetSessionID())
	assert.Equal(t, enums.EResult_Invalid, pkt.GetEResult())

	// Case: Full Proto Packet
	hdr := protocol.NewMsgHdrProtoBuf(enums.EMsg(1), 123, 456)
	pkt = &protocol.Packet{Header: hdr}
	assert.Equal(t, uint64(123), pkt.GetSteamID())
	assert.Equal(t, int32(456), pkt.GetSessionID())
}

func TestPacket_SerializeTo(t *testing.T) {
	t.Run("InvalidHeaderForProto", func(t *testing.T) {
		pkt := &protocol.Packet{
			IsProto: true,
			Header:  &protocol.MsgHdr{}, // Not a ProtoBuf header
		}
		err := pkt.SerializeTo(io.Discard)
		assert.ErrorIs(t, err, protocol.ErrInvalidHeader)
	})

	t.Run("Success", func(t *testing.T) {
		hdr := protocol.NewMsgHdr(enums.EMsg_ChannelEncryptRequest, 0)
		pkt := &protocol.Packet{Header: hdr, Payload: []byte("hi")}
		buf := new(bytes.Buffer)
		err := pkt.SerializeTo(buf)
		assert.NoError(t, err)
		assert.Contains(t, buf.String(), "hi")
	})
}

func TestGCPacket_Roundtrip(t *testing.T) {
	appID := uint32(440)
	msgType := uint32(1001)
	payload := []byte("gc-data")

	t.Run("Proto", func(t *testing.T) {
		p := protocol.NewGCPacket(appID, msgType, payload)
		p.IsProto = true
		p.SourceJobID = 111
		p.TargetJobID = 222

		serialized, err := p.Serialize()
		require.NoError(t, err)

		// ParseGCPacket expects the full serialized data (including MsgType prefix)
		parsed, err := protocol.ParseGCPacket(appID, msgType|protocol.ProtoMask, serialized)
		require.NoError(t, err)

		assert.Equal(t, msgType, parsed.MsgType)
		assert.True(t, parsed.IsProto)
		assert.Equal(t, uint64(111), parsed.SourceJobID)
		assert.Equal(t, uint64(222), parsed.TargetJobID)
		assert.Equal(t, payload, parsed.Payload)
	})

	t.Run("Legacy", func(t *testing.T) {
		p := protocol.NewGCPacket(appID, msgType, payload)
		p.IsProto = false
		p.SourceJobID = 888
		p.TargetJobID = 999

		serialized, err := p.Serialize()
		require.NoError(t, err)

		// Serialize for Legacy includes [Header(18) | Payload] (No MsgType)
		// So we pass the whole buffer
		parsed, err := protocol.ParseGCPacket(appID, msgType, serialized)
		require.NoError(t, err)

		assert.Equal(t, msgType, parsed.MsgType)
		assert.False(t, parsed.IsProto)
		assert.Equal(t, uint64(888), parsed.SourceJobID)
		assert.Equal(t, uint64(999), parsed.TargetJobID)
		assert.Equal(t, payload, parsed.Payload)
	})
}

func TestGCPacket_Errors(t *testing.T) {
	appID := uint32(440)

	t.Run("ParseProtoHeaderLenError", func(t *testing.T) {
		// Can read 4-byte skippedMsgType, but only 2 bytes left, so can't read uint32 hdrLen
		_, err := protocol.ParseGCPacket(appID, protocol.ProtoMask, []byte{1, 2, 3, 4, 5, 6})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "read proto header len")
	})

	t.Run("ParseProtoHeaderReadError", func(t *testing.T) {
		// Prepend 4 bytes for skippedMsgType. HdrLen says 10, but we only give 2 bytes after hdrLen
		data := []byte{1, 2, 3, 4, 10, 0, 0, 0, 1, 2}
		_, err := protocol.ParseGCPacket(appID, protocol.ProtoMask, data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "read proto header")
	})

	t.Run("ParseLegacyHeaderError", func(t *testing.T) {
		// Legacy header must be 18 bytes
		_, err := protocol.ParseGCPacket(appID, 0, []byte{1, 2, 3})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "read legacy header")
	})
}

func TestParsePacket_MissingLineCoverage(t *testing.T) {
	t.Run("Read EMsg Error", func(t *testing.T) {
		// Triggers: if err := binary.Read(r, binary.LittleEndian, &rawEMsg); err != nil
		_, err := protocol.ParsePacket(new(bytes.Buffer))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "read emsg")
	})

	t.Run("Deserialize Header Error", func(t *testing.T) {
		// Triggers: if err := header.Deserialize(r); err != nil
		// Provide EMsg (4 bytes) but nothing for the header
		buf := bytes.NewReader([]byte{0x01, 0x00, 0x00, 0x00})
		_, err := protocol.ParsePacket(buf)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "deserialize header")
	})
}

func TestPacket_NilHeaderGetters(t *testing.T) {
	// Triggers: if p.Header != nil { return p.Header.GetTargetJob() } (and SourceJob)
	p := &protocol.Packet{Header: nil}
	assert.Equal(t, protocol.NoJob, p.GetTargetJobID())
	assert.Equal(t, protocol.NoJob, p.GetSourceJobID())
}

func TestPacket_EHeaderInterfaceNegative(t *testing.T) {
	// Triggers: if eh, ok := p.Header.(EHeader); ok (failing branch)
	// MsgHdr does NOT implement EHeader
	p := &protocol.Packet{Header: &protocol.MsgHdr{}}
	assert.Equal(t, enums.EResult_Invalid, p.GetEResult())
}

func TestPacket_SerializeTo_HeaderError(t *testing.T) {
	// Triggers: if err := p.Header.SerializeTo(w); err != nil
	p := &protocol.Packet{
		Header: protocol.NewMsgHdr(enums.EMsg_ChannelEncryptRequest, 0),
	}
	err := p.SerializeTo(&faultyIO{})
	assert.Error(t, err)
}

func TestTransportMappingRegistry(t *testing.T) {
	randBytes := make([]byte, 8)
	_, _ = rand.Read(randBytes)
	uniqueKey := "CUSTOM_" + hex.EncodeToString(randBytes)

	assert.Equal(t, protocol.TransportTCP, protocol.MapConnectionToTransport("TCP"))
	assert.Equal(t, protocol.TransportWS, protocol.MapConnectionToTransport("WS"))

	assert.Equal(t, protocol.TransportType(uniqueKey), protocol.MapConnectionToTransport(uniqueKey))

	protocol.RegisterTransportMapping(uniqueKey, protocol.TransportType("WEB"))
	assert.Equal(t, protocol.TransportType("WEB"), protocol.MapConnectionToTransport(uniqueKey))
}
