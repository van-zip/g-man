// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package network

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/lemon4ksan/g-man/pkg/log"
)

var _ Connection = (*WS)(nil)

// WS implements a WebSocket-based connection.
// It leverages the gorilla/websocket library for handling the WebSocket protocol details.
type WS struct {
	BaseConnection

	conn   *websocket.Conn
	logger log.Logger

	msgChan    chan NetMessage
	errChan    chan error
	closedChan chan struct{}

	writeMu   sync.Mutex // Protects conn for concurrent writes.
	closeOnce sync.Once  // Ensures Close actions are performed only once.
}

// NewWS establishes a WebSocket connection using the provided context.
// If endpoint does not contain a scheme, it defaults to wss://.
func NewWS(
	ctx context.Context,
	logger log.Logger,
	endpoint, proxyURL string,
	headers http.Header,
) (*WS, error) {
	if !strings.Contains(endpoint, "://") {
		endpoint = "wss://" + endpoint
	}

	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("ws: invalid endpoint URL: %w", err)
	}

	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}

	dialer := &websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		Proxy:            http.ProxyFromEnvironment,
	}

	if proxyURL != "" {
		pu, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("ws: invalid proxy URL: %w", err)
		}

		dialer.Proxy = http.ProxyURL(pu)
	}

	conn, resp, err := dialer.DialContext(ctx, u.String(), headers)
	if resp != nil {
		_ = resp.Body.Close()
	}

	if err != nil {
		return nil, fmt.Errorf("ws: dial failed: %w", err)
	}

	w := &WS{
		BaseConnection: NewBaseConnection("WS"),
		conn:           conn,
		logger:         logger.With(log.String("transport", "WS"), log.String("endpoint", endpoint)),
		msgChan:        make(chan NetMessage, 100),
		errChan:        make(chan error, 10),
		closedChan:     make(chan struct{}),
	}

	go w.readLoop()

	return w, nil
}

// Name returns the transport identifier.
func (w *WS) Name() string { return "WS" }

// Messages returns the incoming message channel.
func (w *WS) Messages() <-chan NetMessage { return w.msgChan }

// Errors returns the transport error channel.
func (w *WS) Errors() <-chan error { return w.errChan }

// Closed returns the connection closure channel.
func (w *WS) Closed() <-chan struct{} { return w.closedChan }

// Send transmits data as a binary message over the WebSocket connection.
func (w *WS) Send(ctx context.Context, data []byte) error {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()

	if w.conn == nil {
		return errors.New("ws: connection closed")
	}

	var err error
	if deadline, ok := ctx.Deadline(); ok {
		err = w.conn.SetWriteDeadline(deadline)
	} else {
		err = w.conn.SetWriteDeadline(time.Now().Add(WriteTimeout))
	}

	if err != nil {
		return err
	}

	return w.conn.WriteMessage(websocket.BinaryMessage, data)
}

// Close sends a standard WebSocket close frame and terminates the connection.
// It is safe to call multiple times.
func (w *WS) Close() error {
	var err error

	w.closeOnce.Do(func() {
		w.writeMu.Lock()
		defer w.writeMu.Unlock()

		if w.conn != nil {
			// Best-effort attempt to send a clean close message.
			msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
			_ = w.conn.WriteMessage(websocket.CloseMessage, msg)
			err = w.conn.Close()
		}
	})

	return err
}

// readLoop runs in a dedicated goroutine, reading messages from the WebSocket.
func (w *WS) readLoop() {
	defer func() {
		_ = w.Close()
		close(w.closedChan)
		close(w.msgChan)
		close(w.errChan)
	}()

	for {
		msgType, data, err := w.conn.ReadMessage()
		if err != nil {
			// Filter out expected close errors to avoid noisy logs.
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				select {
				case w.errChan <- err:
				default:
				}
			}

			return
		}

		if msgType == websocket.BinaryMessage {
			select {
			case w.msgChan <- data:
			case <-w.closedChan:
				return
			}
		}
	}
}
