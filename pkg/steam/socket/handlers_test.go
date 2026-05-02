// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package socket

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"

	pb "github.com/lemon4ksan/g-man/pkg/protobuf/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol/enums"
)

func Test_processSingle_Saturation(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EventChanSize = 1

	sock := NewSocket(cfg)
	defer sock.Close()

	// Fill the channel
	sock.msgCh <- &protocol.Packet{}

	// This should log a warning but not block
	sock.processSingle(bytes.NewReader(packProto(enums.EMsg(1), 0, nil)))

	// This should try again because it's a job response
	go sock.processSingle(bytes.NewReader(packProto(enums.EMsg(1), 123, nil)))

	time.Sleep(10 * time.Millisecond) // Give goroutine time to run
	<-sock.msgCh                      // Drain one

	select {
	case pkt := <-sock.msgCh:
		assert.Equal(t, uint64(123), pkt.GetTargetJobID())
	case <-time.After(150 * time.Millisecond):
		t.Fatal("job response was dropped")
	}
}

func Test_handleService(t *testing.T) {
	sock := NewSocket(DefaultConfig())
	defer sock.Close()

	// Non-proto header case
	sock.handleService(&protocol.Packet{Header: &protocol.MsgHdr{}})

	// Unhandled method case
	hdr := protocol.NewMsgHdrProtoBuf(enums.EMsg_ServiceMethodResponse, 0, 0)
	hdr.Proto.TargetJobName = proto.String("unhandled")
	sock.handleService(&protocol.Packet{Header: hdr})
}

func Test_handleMulti(t *testing.T) {
	sock := NewSocket(DefaultConfig())
	defer sock.Close()

	// Proto unmarshal error
	sock.handleMulti(&protocol.Packet{Payload: []byte{0xFF}})

	// Decompression limit
	data, _ := proto.Marshal(&pb.CMsgMulti{SizeUnzipped: proto.Uint32(200 * 1024 * 1024)})
	sock.handleMulti(&protocol.Packet{Payload: data})

	// Corrupt GZIP
	data, _ = proto.Marshal(&pb.CMsgMulti{SizeUnzipped: proto.Uint32(10), MessageBody: []byte("bad")})
	sock.handleMulti(&protocol.Packet{Payload: data})

	// Sub-packet parse fail (should just continue)
	payload := new(bytes.Buffer)
	binary.Write(payload, binary.LittleEndian, uint32(1)) // Size of 1
	payload.WriteByte(0xFF)                               // Only 1 byte, will fail ParsePacket
	sock.handleMulti(&protocol.Packet{Payload: payload.Bytes()})
}

func TestHandlers_ProcessSingle_Coverage(t *testing.T) {
	t.Run("Parse Failure", func(t *testing.T) {
		sock := NewSocket(DefaultConfig())
		defer sock.Close()

		sock.processSingle(bytes.NewReader([]byte{0xFF, 0xFF}))
	})

	t.Run("Channel Saturated and Timeout", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.EventChanSize = 1

		sock := NewSocket(cfg)
		defer sock.Close()

		// Fill channel
		sock.msgCh <- &protocol.Packet{EMsg: enums.EMsg(1)}

		// Packet without JobID (Hits default drop branch)
		sock.processSingle(bytes.NewReader(packProto(enums.EMsg(2), 0, nil)))

		// Packet WITH JobID (Hits the retry logic and timeout)
		start := time.Now()

		sock.processSingle(bytes.NewReader(packProto(enums.EMsg(3), 12345, nil)))
		assert.True(t, time.Since(start) >= 100*time.Millisecond)
	})

	t.Run("Socket Closed During Process", func(t *testing.T) {
		sock := NewSocket(DefaultConfig())
		close(sock.done) // Pre-close

		// Triggers: case <-s.done:
		sock.processSingle(bytes.NewReader(packProto(enums.EMsg(1), 0, nil)))
	})
}

func TestHandlers_HandleService_Coverage(t *testing.T) {
	sock := NewSocket(DefaultConfig())
	defer sock.Close()

	t.Run("Non-Proto Header", func(t *testing.T) {
		pkt := &protocol.Packet{Header: &protocol.MsgHdr{}}
		sock.handleService(pkt)
	})

	t.Run("Unhandled Method", func(t *testing.T) {
		hdr := protocol.NewMsgHdrProtoBuf(enums.EMsg_ServiceMethodResponse, 0, 0)
		hdr.Proto.TargetJobName = proto.String("NonExistent.Method#1")
		sock.handleService(&protocol.Packet{Header: hdr})
	})
}

func TestHandlers_HandleMulti_Coverage(t *testing.T) {
	t.Run("Unmarshal Failure", func(t *testing.T) {
		sock := NewSocket(DefaultConfig())
		sock.handleMulti(&protocol.Packet{Payload: []byte{0xFF, 0xFF}})
	})

	t.Run("Decompression Failures", func(t *testing.T) {
		sock := NewSocket(DefaultConfig())

		badGzip, _ := proto.Marshal(&pb.CMsgMulti{
			SizeUnzipped: proto.Uint32(100),
			MessageBody:  []byte("not-gzip"),
		})
		sock.handleMulti(&protocol.Packet{Payload: badGzip})

		var buf bytes.Buffer

		gw := gzip.NewWriter(&buf)
		gw.Write([]byte("short"))
		gw.Close()

		mismatch, _ := proto.Marshal(&pb.CMsgMulti{
			SizeUnzipped: proto.Uint32(1000), // Declared 1000, actual "short"
			MessageBody:  buf.Bytes(),
		})
		sock.handleMulti(&protocol.Packet{Payload: mismatch})
	})

	t.Run("Malformed Sub-packet Loop", func(t *testing.T) {
		sock := NewSocket(DefaultConfig())

		payload := new(bytes.Buffer)
		// First sub-packet: Invalid EMsg/Hdr read (triggers binary.Read error to break loop)
		binary.Write(payload, binary.LittleEndian, uint32(1)) // Says size 1
		// Loop will try to read 4 bytes for size next time and fail

		multi, _ := proto.Marshal(&pb.CMsgMulti{
			MessageBody: payload.Bytes(),
		})
		sock.handleMulti(&protocol.Packet{Payload: multi})
	})

	t.Run("Sub-packet Parse Error", func(t *testing.T) {
		sock := NewSocket(DefaultConfig())

		payload := new(bytes.Buffer)
		// Size is 4, but 4 bytes of garbage won't parse as a packet
		binary.Write(payload, binary.LittleEndian, uint32(4))
		payload.Write([]byte{0x00, 0x00, 0x00, 0x00})

		multi, _ := proto.Marshal(&pb.CMsgMulti{
			MessageBody: payload.Bytes(),
		})
		// Triggers: if err != nil { continue } inside loop
		sock.handleMulti(&protocol.Packet{Payload: multi})
	})

	t.Run("Channel Saturation and Context Cancel", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.EventChanSize = 1

		sock := NewSocket(cfg)
		defer sock.Close()

		sock.msgCh <- &protocol.Packet{} // fill

		sub := packProto(enums.EMsg(1), 0, nil)
		payload := new(bytes.Buffer)
		binary.Write(payload, binary.LittleEndian, uint32(len(sub)))
		payload.Write(sub)

		multi, _ := proto.Marshal(&pb.CMsgMulti{MessageBody: payload.Bytes()})
		sock.handleMulti(&protocol.Packet{Payload: multi})

		ctx, cancel := context.WithCancel(context.Background())
		sock.ctx.Store(ctx)
		cancel()

		sock.handleMulti(&protocol.Packet{Payload: multi})
	})
}

func TestHandlers_DecompressPayload_Limit(t *testing.T) {
	sock := NewSocket(DefaultConfig())

	_, err := sock.decompressPayload([]byte{}, 101*1024*1024)
	assert.ErrorIs(t, err, ErrDecompressionLimit)
}
