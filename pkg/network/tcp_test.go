// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package network

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man/pkg/log"
)

type mockFramer struct {
	readFunc  func(r io.Reader) ([]byte, error)
	writeFunc func(w io.Writer, data []byte) error
}

func (m mockFramer) ReadFrame(r io.Reader) ([]byte, error) {
	if m.readFunc != nil {
		return m.readFunc(r)
	}

	return nil, io.EOF
}

func (m mockFramer) WriteFrame(w io.Writer, data []byte) error {
	if m.writeFunc != nil {
		return m.writeFunc(w, data)
	}

	return nil
}

type mockCipher struct {
	encFunc func(data []byte) ([]byte, error)
	decFunc func(data []byte) ([]byte, error)
}

func (m mockCipher) Encrypt(data []byte) ([]byte, error) {
	if m.encFunc != nil {
		return m.encFunc(data)
	}

	return data, nil
}

func (m mockCipher) Decrypt(data []byte) ([]byte, error) {
	if m.decFunc != nil {
		return m.decFunc(data)
	}

	return data, nil
}

func TestTCP_NewTCP_Fail(t *testing.T) {
	_, err := NewTCP(context.Background(), log.Discard, "127.0.0.1:1", "", nil, mockFramer{})
	assert.Error(t, err)

	_, err = NewTCP(context.Background(), log.Discard, "127.0.0.1:1", "", nil, nil)
	assert.ErrorContains(t, err, "framer cannot be nil")
}

func TestTCP_Name(t *testing.T) {
	tcp := &TCP{}
	assert.Equal(t, "TCP", tcp.Name())
}

func TestTCP_ReadLoop_Coverage(t *testing.T) {
	logger := log.Discard

	t.Run("Decryption Branch", func(t *testing.T) {
		s, c := net.Pipe()
		tcp := &TCP{
			conn:           c,
			logger:         logger,
			BaseConnection: NewBaseConnection("TCP"),
			framer: mockFramer{
				readFunc: func(r io.Reader) ([]byte, error) {
					b := make([]byte, 10)
					_, err := r.Read(b)
					return b, err
				},
			},
			msgChan:    make(chan NetMessage, 10),
			errChan:    make(chan error, 10),
			closedChan: make(chan struct{}),
		}

		cipher := mockCipher{
			decFunc: func(data []byte) ([]byte, error) {
				return nil, errors.New("decrypt failed")
			},
		}
		tcp.SetCipher(cipher)

		go tcp.readLoop()

		_, _ = s.Write([]byte("some data!"))

		select {
		case err := <-tcp.Errors():
			assert.ErrorContains(t, err, "tcp: decrypt failed")
		case <-time.After(time.Second):
			t.Fatal("timeout")
		}
	})

	t.Run("Framer Error", func(t *testing.T) {
		_, c := net.Pipe()

		tcp := &TCP{
			conn:   c,
			logger: log.Discard,
			framer: mockFramer{
				readFunc: func(r io.Reader) ([]byte, error) {
					return nil, errors.New("invalid frame")
				},
			},
			msgChan:    make(chan NetMessage, 10),
			errChan:    make(chan error, 10),
			closedChan: make(chan struct{}),
		}
		go tcp.readLoop()

		select {
		case err := <-tcp.Errors():
			assert.ErrorContains(t, err, "invalid frame")
		case <-time.After(time.Second):
			t.Fatal("timeout")
		}
	})
}

func TestTCP_SetCipher(t *testing.T) {
	tcp := &TCP{logger: log.Discard}
	cipher := mockCipher{}
	ok := tcp.SetCipher(cipher)
	assert.True(t, ok)
	assert.NotNil(t, tcp.cipher)
}

func TestTCP_Send_Deadline(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	defer l.Close()

	go func() {
		conn, _ := l.Accept()
		if conn != nil {
			conn.Close()
		}
	}()

	clientConn, err := net.Dial("tcp", l.Addr().String())
	require.NoError(t, err)

	tcp := &TCP{
		conn:           clientConn,
		logger:         log.Discard,
		BaseConnection: NewBaseConnection("TCP"),
		framer:         mockFramer{},
	}
	defer tcp.Close()

	t.Run("Immediate_Context_Cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel before calling Send

		assert.ErrorIs(t, tcp.Send(ctx, []byte("some data")), context.Canceled)
	})

	t.Run("No_Context_Deadline_Branch", func(t *testing.T) {
		assert.NotPanics(t, func() {
			tcp.Send(context.Background(), []byte("data"))
		})
	})
}

func TestIsIgnorableError(t *testing.T) {
	assert.True(t, isIgnorableError(io.EOF))
	assert.True(t, isIgnorableError(net.ErrClosed))
	assert.False(t, isIgnorableError(context.DeadlineExceeded))
	assert.False(t, isIgnorableError(errors.New("random error")))
}
