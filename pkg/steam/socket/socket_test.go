// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package socket

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/lemon4ksan/g-man/pkg/bus"
	"github.com/lemon4ksan/g-man/pkg/log"
	pb "github.com/lemon4ksan/g-man/pkg/protobuf/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol/enums"
	"github.com/lemon4ksan/g-man/pkg/steam/socket/internal/network"
	"github.com/lemon4ksan/g-man/pkg/steam/socket/internal/session"
)

type mockConnection struct {
	network.BaseConnection
	sendFunc        func(ctx context.Context, data []byte) error
	closeFunc       func() error
	setEncryptFunc  func(key []byte) bool
	supportsEncrypt bool
	sentMsgs        chan []byte
}

func newMockConnection() *mockConnection {
	return &mockConnection{
		BaseConnection: network.NewBaseConnection("mock"),
		sendFunc:       func(ctx context.Context, data []byte) error { return nil },
		closeFunc:      func() error { return nil },
		setEncryptFunc: func(key []byte) bool { return false },
		sentMsgs:       make(chan []byte, 10),
	}
}

func (m *mockConnection) Name() string { return "MOCK" }
func (m *mockConnection) Send(ctx context.Context, d []byte) error {
	m.sentMsgs <- d
	return m.sendFunc(ctx, d)
}
func (m *mockConnection) Close() error                     { return m.closeFunc() }
func (m *mockConnection) SetEncryptionKey(key []byte) bool { return m.setEncryptFunc(key) }
func (m *mockConnection) SupportsEncryption() bool         { return m.supportsEncrypt }

// packProto constructs a binary packet with a Protobuf header.
func packProto(eMsg enums.EMsg, jobId uint64, payload []byte) []byte {
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, uint32(eMsg)|0x80000000)
	hdr := &pb.CMsgProtoBufHeader{JobidTarget: proto.Uint64(jobId)}
	hdrBytes, _ := proto.Marshal(hdr)
	_ = binary.Write(buf, binary.LittleEndian, uint32(len(hdrBytes)))
	buf.Write(hdrBytes)
	buf.Write(payload)

	return buf.Bytes()
}

func TestSocket_Initialization(t *testing.T) {
	t.Run("Default Config", func(t *testing.T) {
		cfg := Config{WorkerCount: 0, Dialers: nil} // Force defaults

		sock := NewSocket(cfg)
		defer sock.Close()

		assert.Equal(t, StateDisconnected, sock.State())
		assert.NotNil(t, sock.Bus())
		assert.Equal(t, 1, sock.config.WorkerCount)
		assert.NotNil(t, sock.config.Dialers["tcp"])
	})

	t.Run("With Options", func(t *testing.T) {
		b := bus.New()
		l := log.New(log.DefaultConfig(log.DebugLevel))

		sock := NewSocket(DefaultConfig(), WithBus(b), WithLogger(l))
		defer sock.Close()

		assert.Same(t, b, sock.Bus())
	})
}

func TestEnums_Stringer(t *testing.T) {
	assert.Equal(t, "disconnected", StateDisconnected.String())
	assert.Equal(t, "connecting", StateConnecting.String())
	assert.Equal(t, "connected", StateConnected.String())
	assert.Equal(t, "disconnecting", StateDisconnecting.String())
	assert.Equal(t, "unknown", State(99).String())
}

func TestSocket_ConnectAndDisconnect(t *testing.T) {
	conn := newMockConnection()
	cfg := DefaultConfig()
	cfg.Dialers = map[string]ConnectionDialer{
		"mock": func(ctx context.Context, nh network.Handler, l log.Logger, s string) (network.Connection, error) {
			return conn, nil
		},
	}

	sock := NewSocket(cfg)
	defer sock.Close()

	// Test Connect
	err := sock.Connect(CMServer{Type: "mock", Endpoint: "host"})
	require.NoError(t, err)
	assert.Equal(t, StateConnected, sock.State())
	require.NotNil(t, sock.Session())

	// Test Disconnect
	sock.Disconnect()
	assert.Equal(t, StateDisconnected, sock.State())
	assert.Nil(t, sock.Session())
	sock.Disconnect() // Should be a no-op
}

func TestSocket_ConnectErrors(t *testing.T) {
	sock := NewSocket(DefaultConfig())
	defer sock.Close()

	// Already Connected
	sock.setState(StateConnected)
	err := sock.Connect(CMServer{})
	assert.ErrorIs(t, err, ErrAlreadyConnected)
	sock.setState(StateDisconnected)

	// Already Connecting
	sock.setState(StateConnecting)
	err = sock.Connect(CMServer{})
	assert.ErrorIs(t, err, ErrAlreadyConnecting)
	sock.setState(StateDisconnected)

	// Unsupported Type
	err = sock.Connect(CMServer{Type: "invalid"})
	assert.ErrorIs(t, err, ErrUnsupportedType)

	// Dialer Failure
	cfg := DefaultConfig()
	cfg.Dialers = map[string]ConnectionDialer{
		"fail": func(ctx context.Context, nh network.Handler, l log.Logger, s string) (network.Connection, error) {
			return nil, errors.New("dial error")
		},
	}

	sock = NewSocket(cfg)
	defer sock.Close()

	err = sock.Connect(CMServer{Type: "fail"})
	assert.ErrorContains(t, err, "dial error")
}

func TestSocket_Close(t *testing.T) {
	sock := NewSocket(DefaultConfig())

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		<-sock.Done()
		wg.Done()
	}()

	err := sock.Close()
	require.NoError(t, err)

	// Subsequent calls should do nothing
	err = sock.Close()
	assert.NoError(t, err)

	wg.Wait() // Ensure Done channel was closed
	assert.ErrorIs(t, sock.Send(context.Background(), nil), ErrClosed)
}

func TestSocket_Handlers(t *testing.T) {
	sock := NewSocket(DefaultConfig())
	defer sock.Close()

	// Msg Handler
	sock.RegisterMsgHandler(enums.EMsg_ClientLogon, func(p *protocol.Packet) {})
	sock.handlersMu.RLock()
	_, ok := sock.handlers[enums.EMsg_ClientLogon]
	sock.handlersMu.RUnlock()
	assert.True(t, ok)

	sock.RegisterMsgHandler(enums.EMsg_ClientLogon, nil) // Unregister
	sock.handlersMu.RLock()
	_, ok = sock.handlers[enums.EMsg_ClientLogon]
	sock.handlersMu.RUnlock()
	assert.False(t, ok)

	// Service Handler
	sock.RegisterServiceHandler("Test.Method", func(p *protocol.Packet) {})
	sock.serviceHandlersMu.RLock()
	_, ok = sock.serviceHandlers["Test.Method"]
	sock.serviceHandlersMu.RUnlock()
	assert.True(t, ok)

	sock.ClearHandlers()
	sock.serviceHandlersMu.RLock()
	_, ok = sock.serviceHandlers["Test.Method"]
	sock.serviceHandlersMu.RUnlock()
	assert.False(t, ok)
}

func TestSocket_PanicRecovery(t *testing.T) {
	sock := NewSocket(DefaultConfig())
	defer sock.Close()

	sock.RegisterMsgHandler(enums.EMsg(1), func(p *protocol.Packet) { panic("test") })

	// This should not panic
	sock.routePacket(&protocol.Packet{EMsg: enums.EMsg(1)})
}

func TestSocket_Heartbeat(t *testing.T) {
	conn := newMockConnection()
	cfg := DefaultConfig()
	cfg.Dialers = map[string]ConnectionDialer{
		"mock": func(c context.Context, n network.Handler, l log.Logger, s string) (network.Connection, error) {
			return conn, nil
		},
	}

	sock := NewSocket(cfg)
	defer sock.Close()

	// No-op if not connected
	sock.StartHeartbeat(time.Hour)
	assert.False(t, sock.heartbeatActive.Load())

	require.NoError(t, sock.Connect(CMServer{Type: "mock"}))

	sock.StartHeartbeat(20 * time.Millisecond)
	assert.True(t, sock.heartbeatActive.Load())
	sock.StartHeartbeat(20 * time.Millisecond) // Duplicate call should be ignored

	select {
	case <-conn.sentMsgs:
		// success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("heartbeat not sent")
	}

	// Test heartbeat stops on disconnect
	sock.Disconnect()
	time.Sleep(30 * time.Millisecond) // Allow time for ticker to fire
	assert.False(t, sock.heartbeatActive.Load())
}

func TestSocket_ReconnectLoop(t *testing.T) {
	attempts := atomic.Int32{}
	cfg := DefaultConfig()
	cfg.ReconnectPolicy.MaxAttempts = 2
	cfg.ReconnectPolicy.InitialBackoff = time.Millisecond
	cfg.ReconnectPolicy.MaxBackoff = 5 * time.Millisecond
	cfg.ReconnectPolicy.BackoffFactor = 1.0
	cfg.ReconnectPolicy.ServerSelector = func(s []CMServer) CMServer { return s[0] }
	cfg.Dialers = map[string]ConnectionDialer{
		"fail": func(c context.Context, n network.Handler, l log.Logger, s string) (network.Connection, error) {
			attempts.Add(1)
			return nil, errors.New("fail")
		},
	}

	sock := NewSocket(cfg)
	// Reconnect only triggers if we were previously connected
	sock.setState(StateConnected)
	sock.UpdateServers([]CMServer{{Type: "fail", Endpoint: "localhost"}})

	// Trigger the close
	sock.handleRemoteClose()

	// Wait for the reconnect loop goroutine (spawned via workerWg.Go) to finish
	sock.workerWg.Wait()

	assert.Equal(t, int32(2), attempts.Load(), "Should have attempted reconnection twice")
	assert.Equal(t, StateDisconnected, sock.State())
}

func TestInboundHandler(t *testing.T) {
	sock := NewSocket(DefaultConfig())
	defer sock.Close()

	handler := inboundHandler{sock: sock}

	// OnNetMessage
	go handler.OnNetMessage(packProto(enums.EMsg(1), 0, nil))

	select {
	case <-sock.msgCh:
		// success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("OnNetMessage did not enqueue packet")
	}

	// OnNetError
	sub := sock.Bus().Subscribe(NetworkErrorEvent{})

	handler.OnNetError(errors.New("test"))

	select {
	case <-sub.C():
		// success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("OnNetError did not publish event")
	}

	// OnNetClose
	sock.setState(StateConnected)
	handler.OnNetClose()
	assert.Equal(t, StateDisconnected, sock.State())
	sock.handleRemoteClose() // Should be no-op when already disconnected
}

func TestLoggedSession(t *testing.T) {
	conn := newMockConnection()
	conn.supportsEncrypt = true

	var receivedKey []byte

	conn.setEncryptFunc = func(key []byte) bool { receivedKey = key; return true }

	base := newLoggedSession(session.New(conn), log.Discard)

	require.NoError(t, base.Send(context.Background(), []byte("data")))
	assert.Equal(t, "data", string(<-conn.sentMsgs))

	base.SetEncryptionKey([]byte("key"))
	assert.Equal(t, "key", string(receivedKey))

	require.NoError(t, base.Close())
}

func TestSocket_CongestionCoverage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EventChanSize = 1 // Tiny buffer

	sock := NewSocket(cfg)
	defer sock.Close()

	// 1. Fill the channel
	sock.msgCh <- &protocol.Packet{EMsg: enums.EMsg(1)}

	// 2. Try to process a packet that is NOT a job response.
	// Hits the default: s.logger.Warn("Packet dropped: msgCh saturated")
	sock.processSingle(bytes.NewReader(packProto(enums.EMsg(2), 0, nil)))

	// 3. Try to process a packet that IS a job response (TargetJobID != NoJob).
	// Hits the s.msgCh <- packet with 100ms timeout logic.
	// We run this in a goroutine so it doesn't block the test for 100ms.
	go sock.processSingle(bytes.NewReader(packProto(enums.EMsg(3), 12345, nil)))

	time.Sleep(10 * time.Millisecond)
	<-sock.msgCh // Free up space

	pkt := <-sock.msgCh
	assert.Equal(t, uint64(12345), pkt.GetTargetJobID())
}

func TestSocket_MultiCongestion(t *testing.T) {
	cfg := DefaultConfig()
	cfg.EventChanSize = 1

	sock := NewSocket(cfg)
	defer sock.Close()

	// Fill channel
	sock.msgCh <- &protocol.Packet{EMsg: enums.EMsg(1)}

	// Build a multi-message with two sub-packets
	sub := packProto(enums.EMsg(2), 0, nil)
	payload := new(bytes.Buffer)
	binary.Write(payload, binary.LittleEndian, uint32(len(sub)))
	payload.Write(sub)
	binary.Write(payload, binary.LittleEndian, uint32(len(sub)))
	payload.Write(sub)

	multi, _ := proto.Marshal(&pb.CMsgMulti{
		SizeUnzipped: proto.Uint32(uint32(payload.Len())),
		MessageBody:  payload.Bytes(),
	})

	// This hits the default: branch in handleMulti for sub-packets
	sock.handleMulti(&protocol.Packet{Payload: multi})
}

func TestSocket_InternalState(t *testing.T) {
	sock := NewSocket(DefaultConfig())
	defer sock.Close()

	t.Run("UpdateServers and IsConnected", func(t *testing.T) {
		servers := []CMServer{{Endpoint: "1.2.3.4", Type: "tcp"}}
		sock.UpdateServers(servers)

		sock.serversMu.RLock()
		assert.Equal(t, servers, sock.servers)
		sock.serversMu.RUnlock()

		assert.False(t, sock.IsConnected())
		sock.setState(StateConnected)
		assert.True(t, sock.IsConnected())
	})

	t.Run("Handler Deletion", func(t *testing.T) {
		// Msg Handlers
		sock.RegisterMsgHandler(enums.EMsg_ClientLogon, func(p *protocol.Packet) {})
		sock.RegisterMsgHandler(enums.EMsg_ClientLogon, nil) // Delete

		sock.handlersMu.RLock()
		assert.NotContains(t, sock.handlers, enums.EMsg_ClientLogon)
		sock.handlersMu.RUnlock()

		// Service Handlers
		sock.RegisterServiceHandler("A.B#1", func(p *protocol.Packet) {})
		sock.RegisterServiceHandler("A.B#1", nil) // Delete

		sock.serviceHandlersMu.RLock()
		assert.NotContains(t, sock.serviceHandlers, "A.B#1")
		sock.serviceHandlersMu.RUnlock()
	})
}

func TestSocket_Reconnect_Fallbacks(t *testing.T) {
	cfg := DefaultConfig()
	// Mock policy where selector returns empty server to hit fallback logic
	cfg.ReconnectPolicy.ServerSelector = func(servers []CMServer) CMServer { return CMServer{} }
	cfg.ReconnectPolicy.MaxAttempts = 1

	dialedEndpoint := ""
	cfg.Dialers = map[string]ConnectionDialer{
		"tcp": func(ctx context.Context, nh network.Handler, l log.Logger, endpoint string) (network.Connection, error) {
			dialedEndpoint = endpoint
			return nil, errors.New("fail")
		},
	}

	sock := NewSocket(cfg)
	sock.lastServer = CMServer{Endpoint: "last_resort:1234", Type: "tcp"}
	sock.setState(StateConnected) // Must be connected to trigger reconnect

	// Trigger reconnect
	sock.handleRemoteClose()
	sock.workerWg.Wait()

	// Triggers: if targetServer.Endpoint == "" { targetServer = s.lastServer }
	assert.Equal(t, "last_resort:1234", dialedEndpoint)
}

func TestLoggedSession_EncryptionFailure(t *testing.T) {
	conn := newMockConnection()
	conn.supportsEncrypt = false
	conn.setEncryptFunc = func(key []byte) bool { return false }

	ls := newLoggedSession(session.New(conn), log.Discard)

	// This branch triggers: l.logger.Warn("Channel encryption skipped...")
	result := ls.SetEncryptionKey([]byte("key"))
	assert.False(t, result)
}
