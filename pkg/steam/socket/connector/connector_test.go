// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package connector_test

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/network"
	"github.com/lemon4ksan/g-man/pkg/steam/socket/connector"
)

type mockConnection struct {
	network.BaseConnection
	sendFunc      func(ctx context.Context, data []byte) error
	closeFunc     func() error
	setCipherFunc func(c network.Cipher) bool

	msgChan    chan network.NetMessage
	errChan    chan error
	closedChan chan struct{}
}

func newMockConnection() *mockConnection {
	return &mockConnection{
		msgChan:    make(chan network.NetMessage, 10),
		errChan:    make(chan error, 10),
		closedChan: make(chan struct{}),
	}
}

func (m *mockConnection) Send(ctx context.Context, data []byte) error { return m.sendFunc(ctx, data) }
func (m *mockConnection) Close() error                                { return m.closeFunc() }
func (m *mockConnection) Name() string                                { return "mock" }
func (m *mockConnection) SetCipher(c network.Cipher) bool             { return m.setCipherFunc(c) }
func (m *mockConnection) Messages() <-chan network.NetMessage         { return m.msgChan }
func (m *mockConnection) Errors() <-chan error                        { return m.errChan }
func (m *mockConnection) Closed() <-chan struct{}                     { return m.closedChan }

func TestConnector_Initialization(t *testing.T) {
	c := connector.New(connector.DefaultConfig(), log.Discard)
	defer c.Close()

	assert.NotNil(t, c)
}

func TestConnector_Connect(t *testing.T) {
	t.Run("Successful Connection", func(t *testing.T) {
		var conn *mockConnection

		dialers := map[string]connector.Dialer{
			"mock": func(ctx context.Context, l log.Logger, ep, _ string, _ http.Header) (network.Connection, error) {
				if conn == nil {
					conn = newMockConnection()
					conn.closeFunc = func() error { return nil }
				}

				return conn, nil
			},
		}

		cfg := connector.DefaultConfig()
		cfg.Dialers = dialers

		c := connector.New(cfg, log.Discard)

		err := c.Connect(context.Background(), connector.CMServer{Type: "mock", Endpoint: "localhost"})
		assert.NoError(t, err)

		// Re-connect should close previous
		closed := atomic.Bool{}
		conn.closeFunc = func() error { closed.Store(true); return nil }

		err = c.Connect(context.Background(), connector.CMServer{Type: "mock", Endpoint: "localhost:2"})
		assert.NoError(t, err)
		assert.True(t, closed.Load())
	})

	t.Run("Unsupported Type", func(t *testing.T) {
		c := connector.New(connector.DefaultConfig(), log.Discard)
		err := c.Connect(context.Background(), connector.CMServer{Type: "invalid"})
		assert.ErrorIs(t, err, connector.ErrUnsupportedType)
	})

	t.Run("Dialer Error", func(t *testing.T) {
		dialers := map[string]connector.Dialer{
			"fail": func(ctx context.Context, l log.Logger, ep, _ string, _ http.Header) (network.Connection, error) {
				return nil, errors.New("dial failed")
			},
		}
		cfg := connector.DefaultConfig()
		cfg.Dialers = dialers
		c := connector.New(cfg, log.Discard)

		err := c.Connect(context.Background(), connector.CMServer{Type: "fail"})
		assert.ErrorContains(t, err, "dial failed")
	})

	t.Run("Concurrent Connection Attempt", func(t *testing.T) {
		start := make(chan struct{})
		dialers := map[string]connector.Dialer{
			"slow": func(ctx context.Context, l log.Logger, ep, _ string, _ http.Header) (network.Connection, error) {
				<-start
				return newMockConnection(), nil
			},
		}
		cfg := connector.DefaultConfig()
		cfg.Dialers = dialers
		c := connector.New(cfg, log.Discard)

		go func() { _ = c.Connect(context.Background(), connector.CMServer{Type: "slow"}) }()

		time.Sleep(20 * time.Millisecond) // Let it enter the dialer

		err := c.Connect(context.Background(), connector.CMServer{Type: "slow"})
		assert.ErrorIs(t, err, connector.ErrAlreadyConnecting)

		close(start) // Cleanup
	})
}

func TestConnector_Send(t *testing.T) {
	c := connector.New(connector.DefaultConfig(), log.Discard)

	// Send when disconnected
	err := c.Send(context.Background(), []byte("hi"))
	assert.ErrorIs(t, err, connector.ErrDisconnected)

	// Send when connected
	sent := atomic.Bool{}

	var conn *mockConnection

	dialers := map[string]connector.Dialer{
		"mock": func(ctx context.Context, l log.Logger, ep, _ string, _ http.Header) (network.Connection, error) {
			if conn == nil {
				conn = newMockConnection()
				conn.sendFunc = func(ctx context.Context, data []byte) error {
					sent.Store(true)
					return nil
				}
			}

			return conn, nil
		},
	}
	cfg := connector.DefaultConfig()
	cfg.Dialers = dialers
	c = connector.New(cfg, log.Discard)
	_ = c.Connect(context.Background(), connector.CMServer{Type: "mock"})

	err = c.Send(context.Background(), []byte("payload"))
	assert.NoError(t, err)
	assert.True(t, sent.Load())
}

func TestConnector_Encryption(t *testing.T) {
	cipherReceived := atomic.Pointer[network.Cipher]{}

	var conn *mockConnection

	dialers := map[string]connector.Dialer{
		"mock": func(ctx context.Context, l log.Logger, ep, _ string, _ http.Header) (network.Connection, error) {
			if conn == nil {
				conn = newMockConnection()
				conn.setCipherFunc = func(c network.Cipher) bool {
					cipherReceived.Store(&c)
					return true
				}
			}

			return conn, nil
		},
	}
	cfg := connector.DefaultConfig()
	cfg.Dialers = dialers
	c := connector.New(cfg, log.Discard)
	_ = c.Connect(context.Background(), connector.CMServer{Type: "mock"})

	ok := c.SetEncryptionKey([]byte("secret"))
	assert.True(t, ok)
	assert.NotNil(t, cipherReceived.Load())
}

func TestConnector_ReconnectLoop(t *testing.T) {
	t.Run("Exhaust Attempts", func(t *testing.T) {
		dialCount := atomic.Int32{}

		dialers := map[string]connector.Dialer{
			"fail": func(ctx context.Context, l log.Logger, ep, _ string, _ http.Header) (network.Connection, error) {
				dialCount.Add(1)
				return nil, errors.New("perma-fail")
			},
		}

		policy := connector.DefaultReconnectPolicy()
		policy.MaxAttempts = 2
		policy.InitialBackoff = time.Millisecond
		policy.BackoffFactor = 1.0

		cfg := connector.Config{
			Dialers:         dialers,
			ReconnectPolicy: policy,
			ConnectTimeout:  time.Second,
		}

		c := connector.New(cfg, log.Discard)
		c.UpdateServers([]connector.CMServer{{Type: "fail", Endpoint: "ep1"}})

		// Initial connect to set "lastServer"
		_ = c.Connect(context.Background(), connector.CMServer{Type: "fail", Endpoint: "ep1"})
	})
}

func TestConnector_Lifecycle(t *testing.T) {
	c := connector.New(connector.DefaultConfig(), log.Discard)

	// Close should be idempotent
	err := c.Close()
	assert.NoError(t, err)
	err = c.Close()
	assert.NoError(t, err)

	// Send after close
	err = c.Send(context.Background(), []byte("fail"))
	assert.ErrorIs(t, err, connector.ErrClosed)
}

func TestConnector_ErrorsAndHelpers(t *testing.T) {
	// IsRetriable tests
	assert.False(t, connector.ErrClosed.IsRetriable())
	assert.True(t, connector.ErrDisconnected.IsRetriable())
	assert.True(t, connector.ErrAlreadyConnecting.IsRetriable())
	assert.False(t, connector.ErrUnsupportedType.IsRetriable())
	assert.False(t, connector.ErrReconnectionFailed.IsRetriable())
	assert.Equal(t, "connector: instance is permanently closed", connector.ErrClosed.Error())

	// DefaultReconnectPolicy tests
	policy := connector.DefaultReconnectPolicy()
	assert.Equal(t, connector.CMServer{}, policy.ServerSelector(nil))

	servers := []connector.CMServer{{Endpoint: "ep1"}, {Endpoint: "ep2"}}
	assert.Equal(t, "ep1", policy.ServerSelector(servers).Endpoint)

	// Accessors
	c := connector.New(connector.DefaultConfig(), log.Discard)
	defer c.Close()

	assert.NotNil(t, c.Done())
	assert.NotNil(t, c.C())
	assert.False(t, c.IsConnected())
}

type nonEncryptableConnection struct {
	network.BaseConnection
	msgChan    chan network.NetMessage
	errChan    chan error
	closedChan chan struct{}
}

func (m *nonEncryptableConnection) Send(ctx context.Context, data []byte) error { return nil }
func (m *nonEncryptableConnection) Close() error                                { return nil }
func (m *nonEncryptableConnection) Name() string                                { return "non-enc" }
func (m *nonEncryptableConnection) Messages() <-chan network.NetMessage         { return m.msgChan }
func (m *nonEncryptableConnection) Errors() <-chan error                        { return m.errChan }
func (m *nonEncryptableConnection) Closed() <-chan struct{}                     { return m.closedChan }

func TestConnector_SetEncryptionKey_Failures(t *testing.T) {
	dialers := map[string]connector.Dialer{
		"non-enc": func(ctx context.Context, l log.Logger, ep, _ string, _ http.Header) (network.Connection, error) {
			conn := &nonEncryptableConnection{
				msgChan:    make(chan network.NetMessage, 1),
				errChan:    make(chan error, 1),
				closedChan: make(chan struct{}),
			}

			return conn, nil
		},
	}
	cfg := connector.DefaultConfig()
	cfg.Dialers = dialers

	c := connector.New(cfg, log.Discard)
	defer c.Close()

	// Before connect
	assert.False(t, c.SetEncryptionKey([]byte("secret")))

	// After connect
	err := c.Connect(context.Background(), connector.CMServer{Type: "non-enc", Endpoint: "localhost"})
	assert.NoError(t, err)
	assert.False(t, c.SetEncryptionKey([]byte("secret")))
}

func TestConnector_Disconnect_Coverage(t *testing.T) {
	c := connector.New(connector.DefaultConfig(), log.Discard)
	// Disconnect when not connected
	assert.NoError(t, c.Disconnect())
}

func TestConnector_ReconnectionAndMonitoring(t *testing.T) {
	// Reconnection loop execution and monitoring error channels
	dialCount := atomic.Int32{}

	var conn *mockConnection

	dialers := map[string]connector.Dialer{
		"mock": func(ctx context.Context, l log.Logger, ep, _ string, _ http.Header) (network.Connection, error) {
			count := dialCount.Add(1)
			if count == 1 {
				conn = newMockConnection()
				conn.closeFunc = func() error { return nil }
				return conn, nil
			}

			// Subsequent reconnects fail
			return nil, errors.New("reconnect fail")
		},
	}

	policy := connector.DefaultReconnectPolicy()
	policy.MaxAttempts = 3
	policy.InitialBackoff = time.Millisecond
	policy.BackoffFactor = 1.0

	cfg := connector.Config{
		Dialers:         dialers,
		ReconnectPolicy: policy,
		ConnectTimeout:  time.Second,
	}

	c := connector.New(cfg, log.Discard)
	defer c.Close()

	// Set initial server list
	c.UpdateServers([]connector.CMServer{{Type: "mock", Endpoint: "ep1"}})

	err := c.Connect(context.Background(), connector.CMServer{Type: "mock", Endpoint: "ep1"})
	assert.NoError(t, err)
	assert.True(t, c.IsConnected())

	// Send an error to connection Errors()
	conn.errChan <- errors.New("mock transport error")

	// Trigger connection drop
	close(conn.closedChan)

	// Wait for reconnection loop to exhaust attempts (attempts = 3, initial backoff 1ms)
	time.Sleep(100 * time.Millisecond)

	assert.False(t, c.IsConnected())
	assert.GreaterOrEqual(t, dialCount.Load(), int32(3))
}

func TestConnector_MonitorConnection_Channels(t *testing.T) {
	var conn *mockConnection

	dialers := map[string]connector.Dialer{
		"mock": func(ctx context.Context, l log.Logger, ep, _ string, _ http.Header) (network.Connection, error) {
			conn = newMockConnection()
			conn.closeFunc = func() error { return nil }
			return conn, nil
		},
	}
	cfg := connector.DefaultConfig()
	cfg.Dialers = dialers

	c := connector.New(cfg, log.Discard)
	defer c.Close()

	err := c.Connect(context.Background(), connector.CMServer{Type: "mock", Endpoint: "ep"})
	assert.NoError(t, err)

	// 1. Send valid message
	conn.msgChan <- []byte("payload")

	select {
	case msg := <-c.C():
		assert.Equal(t, []byte("payload"), msg)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timeout waiting for message")
	}

	// 2. Trigger Messages channel close (!ok)
	close(conn.msgChan)
	time.Sleep(20 * time.Millisecond)

	// 3. Trigger Errors channel close (!ok)
	close(conn.errChan)
	time.Sleep(20 * time.Millisecond)
}

func TestConnector_Reconnection_Interrupt(t *testing.T) {
	dialCount := atomic.Int32{}

	var conn *mockConnection

	dialers := map[string]connector.Dialer{
		"mock": func(ctx context.Context, l log.Logger, ep, _ string, _ http.Header) (network.Connection, error) {
			dialCount.Add(1)

			conn = newMockConnection()
			conn.closeFunc = func() error { return nil }

			return conn, nil
		},
		"fail": func(ctx context.Context, l log.Logger, ep, _ string, _ http.Header) (network.Connection, error) {
			dialCount.Add(1)
			return nil, errors.New("fail")
		},
	}

	policy := connector.DefaultReconnectPolicy()
	policy.MaxAttempts = 10
	policy.InitialBackoff = 50 * time.Millisecond
	policy.BackoffFactor = 2.0

	cfg := connector.Config{
		Dialers:         dialers,
		ReconnectPolicy: policy,
		ConnectTimeout:  time.Second,
	}

	c := connector.New(cfg, log.Discard)
	c.UpdateServers([]connector.CMServer{{Type: "fail", Endpoint: "ep1"}})

	err := c.Connect(context.Background(), connector.CMServer{Type: "mock", Endpoint: "ep1"})
	assert.NoError(t, err)

	// Drop connection
	close(conn.closedChan)
	time.Sleep(10 * time.Millisecond)

	// Close connector while reconnection is waiting/backing off
	err = c.Close()
	assert.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
	// Reconnect loop should exit early due to ctx.Done()
	assert.Less(t, dialCount.Load(), int32(5))
}
