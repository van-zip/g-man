// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package network

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/proxy"

	"github.com/lemon4ksan/g-man/pkg/log"
)

const (
	// ReadTimeout is the maximum duration to wait for data before closing the connection.
	ReadTimeout = 60 * time.Second
	// WriteTimeout is the default duration to wait for a write to complete.
	WriteTimeout = 5 * time.Second
)

// Compile-time checks to ensure interface satisfaction.
var (
	_ Connection  = (*TCP)(nil)
	_ Encryptable = (*TCP)(nil)
)

// TCP implements the Connection interface for stream-oriented protocols.
// It relies on a Framer to extract discrete messages from the byte stream,
// and optionally uses a Cipher for encryption.
type TCP struct {
	BaseConnection
	conn   net.Conn
	logger log.Logger
	framer Framer

	msgChan    chan NetMessage
	errChan    chan error
	closedChan chan struct{}

	writeMu sync.Mutex   // Ensures atomic writes.
	keyMu   sync.RWMutex // Protects cipher for concurrent reads/writes.
	cipher  Cipher
}

// NewTCP establishes a TCP connection to the given endpoint and starts its read loop.
func NewTCP(
	ctx context.Context,
	logger log.Logger,
	endpoint, proxyURL string,
	_ http.Header,
	framer Framer,
) (*TCP, error) {
	if framer == nil {
		return nil, errors.New("tcp: framer cannot be nil")
	}

	var conn net.Conn

	if proxyURL != "" {
		u, err := url.Parse(proxyURL)
		if err != nil {
			return nil, fmt.Errorf("tcp: invalid proxy URL: %w", err)
		}

		dialer, err := proxy.FromURL(u, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("tcp: failed to create proxy dialer: %w", err)
		}

		contextDialer, ok := dialer.(proxy.ContextDialer)
		if !ok {
			// Fallback for dialers that don't implement ContextDialer
			conn, err = dialer.Dial("tcp", endpoint)
		} else {
			conn, err = contextDialer.DialContext(ctx, "tcp", endpoint)
		}

		if err != nil {
			return nil, fmt.Errorf("tcp: dial failed: %w", err)
		}
	} else {
		var err error

		dialer := &net.Dialer{}

		conn, err = dialer.DialContext(ctx, "tcp", endpoint)
		if err != nil {
			return nil, fmt.Errorf("tcp: dial failed: %w", err)
		}
	}

	t := &TCP{
		BaseConnection: NewBaseConnection("TCP"),
		conn:           conn,
		logger:         logger.With(log.String("transport", "TCP"), log.String("endpoint", endpoint)),
		framer:         framer,
		msgChan:        make(chan NetMessage, 100),
		errChan:        make(chan error, 10),
		closedChan:     make(chan struct{}),
	}

	go t.readLoop()

	return t, nil
}

// Name returns the "TCP" string.
func (t *TCP) Name() string { return "TCP" }

// Messages returns the incoming message channel.
func (t *TCP) Messages() <-chan NetMessage { return t.msgChan }

// Errors returns the transport error channel.
func (t *TCP) Errors() <-chan error { return t.errChan }

// Closed returns the connection closure channel.
func (t *TCP) Closed() <-chan struct{} { return t.closedChan }

// SetCipher enables symmetric encryption for all subsequent messages.
func (t *TCP) SetCipher(c Cipher) bool {
	t.keyMu.Lock()
	t.cipher = c
	t.keyMu.Unlock()
	t.logger.Debug("Encryption enabled")

	return true
}

// Send encrypts (if a cipher is set) and frames the data before sending it over the TCP socket.
func (t *TCP) Send(ctx context.Context, data []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	t.keyMu.RLock()
	cipher := t.cipher
	t.keyMu.RUnlock()

	var err error
	if cipher != nil {
		data, err = cipher.Encrypt(data)
		if err != nil {
			return fmt.Errorf("tcp: encrypt failed: %w", err)
		}
	}

	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	if deadline, ok := ctx.Deadline(); ok {
		if err := t.conn.SetWriteDeadline(deadline); err != nil {
			return err
		}
	} else {
		if err := t.conn.SetWriteDeadline(time.Now().Add(WriteTimeout)); err != nil {
			return err
		}
	}

	if err := t.framer.WriteFrame(t.conn, data); err != nil {
		return fmt.Errorf("tcp: write frame failed: %w", err)
	}

	return nil
}

// Close terminates the underlying TCP connection.
func (t *TCP) Close() error {
	if t.conn == nil {
		return nil
	}

	return t.conn.Close()
}

// readLoop runs in a dedicated goroutine, continuously reading and decoding packets.
func (t *TCP) readLoop() {
	defer func() {
		_ = t.conn.Close()
		close(t.closedChan)
		close(t.msgChan)
		close(t.errChan)
	}()

	reader := bufio.NewReaderSize(t.conn, 64*1024)

	for {
		if err := t.conn.SetReadDeadline(time.Now().Add(ReadTimeout)); err != nil {
			if !isIgnorableError(err) {
				select {
				case t.errChan <- err:
				default:
				}
			}

			return
		}

		payload, err := t.framer.ReadFrame(reader)
		if err != nil {
			if !isIgnorableError(err) {
				select {
				case t.errChan <- err:
				default:
				}
			}

			return
		}

		t.keyMu.RLock()
		cipher := t.cipher
		t.keyMu.RUnlock()

		if cipher != nil {
			payload, err = cipher.Decrypt(payload)
			if err != nil {
				select {
				case t.errChan <- fmt.Errorf("tcp: decrypt failed: %w", err):
				default:
				}

				// Don't return, as this might be a single corrupt packet.
				// Depending on the protocol, you might want to continue or disconnect.
				continue
			}
		}

		select {
		case t.msgChan <- payload:
		case <-t.closedChan:
			return
		}
	}
}

// isIgnorableError checks for errors that are expected during a normal connection closure.
func isIgnorableError(err error) bool {
	if err == nil {
		return true
	}

	// Robust check for closure/EOF across different platforms/types of conns
	if errors.Is(err, net.ErrClosed) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) {
		return true
	}

	s := err.Error()

	return strings.Contains(s, "use of closed") || strings.Contains(s, "closed pipe")
}
