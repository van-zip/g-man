// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package protocol_test

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"

	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol/enums"
)

func TestHeaders_Getters(t *testing.T) {
	t.Run("MsgHdr", func(t *testing.T) {
		h := protocol.NewMsgHdr(enums.EMsg_ChannelEncryptRequest, 1)
		h.SourceJobID = 2
		assert.Equal(t, uint64(1), h.GetTargetJob())
		assert.Equal(t, uint64(2), h.GetSourceJob())
	})

	t.Run("MsgHdrExtended", func(t *testing.T) {
		h := protocol.NewMsgHdrExtended(enums.EMsg(100), 76561198000, 123)
		h.TargetJobID = 1
		h.SourceJobID = 2
		assert.Equal(t, uint64(1), h.GetTargetJob())
		assert.Equal(t, uint64(2), h.GetSourceJob())
		assert.Equal(t, uint64(76561198000), h.GetSteamID())
		assert.Equal(t, int32(123), h.GetSessionID())
	})

	t.Run("MsgHdrProtoBuf", func(t *testing.T) {
		h := protocol.NewMsgHdrProtoBuf(enums.EMsg(200), 76561198000, 456)
		h.Proto.JobidTarget = proto.Uint64(1)
		h.Proto.JobidSource = proto.Uint64(2)
		h.Proto.Eresult = proto.Int32(int32(enums.EResult_OK))

		assert.Equal(t, uint64(1), h.GetTargetJob())
		assert.Equal(t, uint64(2), h.GetSourceJob())
		assert.Equal(t, uint64(76561198000), h.GetSteamID())
		assert.Equal(t, int32(456), h.GetSessionID())
		assert.Equal(t, enums.EResult_OK, h.GetEResult())
	})
}

func TestMsgHdr_Deserialize_Error(t *testing.T) {
	h := &protocol.MsgHdr{}
	err := h.Deserialize(bytes.NewReader([]byte{1, 2, 3})) // Too short for 16 bytes
	assert.Error(t, err)
}

func TestMsgHdrExtended_Deserialize_Errors(t *testing.T) {
	t.Run("ShortRead", func(t *testing.T) {
		h := &protocol.MsgHdrExtended{}
		err := h.Deserialize(bytes.NewReader([]byte{1}))
		assert.Error(t, err)
	})

	t.Run("InvalidSize", func(t *testing.T) {
		data := make([]byte, 32)
		data[0] = 20 // Expected 36
		h := &protocol.MsgHdrExtended{}
		err := h.Deserialize(bytes.NewReader(data))
		assert.ErrorIs(t, err, protocol.ErrInvalidHeader)
	})

	t.Run("InvalidVersion", func(t *testing.T) {
		data := make([]byte, 32)
		data[0] = 36
		data[1] = 9 // Version mismatch
		h := &protocol.MsgHdrExtended{}
		err := h.Deserialize(bytes.NewReader(data))
		assert.ErrorIs(t, err, protocol.ErrInvalidHeader)
	})

	t.Run("InvalidCanary", func(t *testing.T) {
		data := make([]byte, 32)
		data[0] = 36
		data[1] = 2
		data[19] = 0xAA // Canary mismatch
		h := &protocol.MsgHdrExtended{}
		err := h.Deserialize(bytes.NewReader(data))
		assert.ErrorIs(t, err, protocol.ErrInvalidHeader)
	})
}

func TestMsgHdrProtoBuf_Deserialize_Errors(t *testing.T) {
	t.Run("ReadLenError", func(t *testing.T) {
		h := &protocol.MsgHdrProtoBuf{}
		err := h.Deserialize(bytes.NewReader([]byte{1}))
		assert.Error(t, err)
	})

	t.Run("ReadBodyError", func(t *testing.T) {
		h := &protocol.MsgHdrProtoBuf{}
		// Says length 100, but provides 0
		err := h.Deserialize(bytes.NewReader([]byte{100, 0, 0, 0}))
		assert.Error(t, err)
	})
}

func TestMsgHdrProtoBuf_Deserialize_Limit(t *testing.T) {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, uint32(protocol.MaxHeaderSize+1))

	h := &protocol.MsgHdrProtoBuf{}
	err := h.Deserialize(buf)
	assert.ErrorIs(t, err, protocol.ErrHeaderTooLarge)
}

func TestMsgHdrProtoBuf_Serialize_WriteError(t *testing.T) {
	h := protocol.NewMsgHdrProtoBuf(enums.EMsg(1), 0, 0)
	err := h.SerializeTo(&faultyIO{})
	assert.ErrorIs(t, err, io.ErrShortWrite)
}

func TestMsgHdr_Serialize_WriteError(t *testing.T) {
	h := protocol.NewMsgHdr(enums.EMsg(1), 0)
	err := h.SerializeTo(&faultyIO{})
	assert.ErrorIs(t, err, io.ErrShortWrite)
}

func TestMsgHdrExtended_Serialize_WriteError(t *testing.T) {
	h := protocol.NewMsgHdrExtended(enums.EMsg(1), 0, 0)
	err := h.SerializeTo(&faultyIO{})
	assert.ErrorIs(t, err, io.ErrShortWrite)
}
