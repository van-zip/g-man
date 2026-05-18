// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package network

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man/pkg/log"
)

func TestWS_NewWS(t *testing.T) {
	// Attempt to dial a bad endpoint
	_, err := NewWS(context.Background(), log.Discard, "invalid:80", "")
	assert.Error(t, err)
}

func TestWS_Send_Closed(t *testing.T) {
	ws := &WS{conn: nil}
	err := ws.Send(context.Background(), []byte("data"))
	assert.ErrorContains(t, err, "connection closed")
}

func TestWS_Send_Deadline(t *testing.T) {
	ws := &WS{conn: nil} // triggers the 'conn == nil' check
	err := ws.Send(context.Background(), []byte("data"))
	assert.Error(t, err)
}

func TestWS_ReadLoop(t *testing.T) {
	upgrader := websocket.Upgrader{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}

		// Send Text (Ignored)
		_ = conn.WriteMessage(websocket.TextMessage, []byte("text"))
		// Send Binary (Processed)
		_ = conn.WriteMessage(websocket.BinaryMessage, []byte("bin"))
		// Keep open until client closes
		time.Sleep(100 * time.Millisecond)
		conn.Close()
	}))
	defer server.Close()

	// NewWS forces wss://. We manually construct for the test server (ws://)
	endpoint := strings.TrimPrefix(server.URL, "http://")

	t.Run("Dial Success and Read Binary", func(t *testing.T) {
		// We can't use NewWS directly because it forces WSS.
		// We'll test the logic by mocking the conn or adjusting the test.
		u := url.URL{Scheme: "ws", Host: endpoint, Path: "/cmsocket/"}
		conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		require.NoError(t, err)

		ws := &WS{
			BaseConnection: NewBaseConnection("WS"),
			conn:           conn,
			logger:         log.Discard,
			msgChan:        make(chan NetMessage, 10),
			errChan:        make(chan error, 10),
			closedChan:     make(chan struct{}),
		}

		go ws.readLoop()
		defer ws.Close()

		select {
		case msg := <-ws.Messages():
			assert.Equal(t, NetMessage("bin"), msg)
		case <-time.After(2 * time.Second):
			t.Fatal("timeout")
		}
	})

	t.Run("NewWS Handshake Failure", func(t *testing.T) {
		// This hits the error branch in NewWS
		_, err := NewWS(context.Background(), log.Discard, "localhost:1", "")
		assert.Error(t, err)
	})
}

func TestWS_Close_MultipleTimes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, _ := upgrader.Upgrade(w, r, nil)
		_ = conn.Close()
	}))
	defer server.Close()

	// Use ws:// for the test server
	endpoint := strings.TrimPrefix(server.URL, "http://")
	u := url.URL{Scheme: "ws", Host: endpoint, Path: "/cmsocket/"}

	// Dial manually to ensure we have a valid connection for the Close test
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)

	ws := &WS{
		BaseConnection: NewBaseConnection("WS"),
		conn:           conn,
		logger:         log.Discard,
		msgChan:        make(chan NetMessage, 10),
		errChan:        make(chan error, 10),
		closedChan:     make(chan struct{}),
	}

	// First close
	err = ws.Close()
	assert.NoError(t, err)

	// Second call (hits sync.Once and should return immediately without panic)
	err = ws.Close()
	assert.NoError(t, err)
}
