// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package network

import (
	"context"
	"sync/atomic"
)

// globalConnectionID is an atomic counter used to generate unique connection IDs.
var globalConnectionID atomic.Int64

// NetMessage represents a complete, raw binary message received from the network.
// It is a slice of bytes that a Handler can process.
type NetMessage []byte

// Connection defines the standard interface that all network connection types must implement.
// It provides methods for sending data, closing the connection, and event notification via channels.
type Connection interface {
	// Send transmits the provided data over the connection. It is
	// the responsibility of the implementation to handle message
	// framing and encryption. This method must be safe for concurrent use.
	//
	// Implementation must respect the context; if the context is canceled before
	// the write is complete, the operation must return ctx.Err().
	Send(ctx context.Context, data []byte) error

	// Close gracefully terminates the connection and releases any associated resources.
	// This method should be idempotent (safe to call multiple times).
	Close() error

	// ID returns a unique identifier for this connection instance.
	// IDs are guaranteed to be unique across all connections created
	// during the program's lifetime.
	ID() int64

	// Name returns the protocol name (e.g., "TCP", "WS") for this connection.
	Name() string

	// Messages returns a channel that receives incoming raw network messages.
	// The channel is closed when the connection is terminated.
	Messages() <-chan NetMessage

	// Errors returns a channel that receives non-fatal transport errors.
	// The channel is closed when the connection is terminated.
	Errors() <-chan error

	// Closed returns a channel that is closed when the connection is terminated.
	Closed() <-chan struct{}
}

// Encryptable is an optional interface that connections can implement to support
// session-based symmetric encryption.
type Encryptable interface {
	// SetEncryptionKey provides the connection with the secret key used to
	// encrypt outgoing messages and decrypt incoming ones.
	SetEncryptionKey(key []byte) bool
}

// BaseConnection provides common functionality and state shared by all connection implementations.
// It should be embedded in concrete connection types (e.g., TCPConnection).
type BaseConnection struct {
	id   int64
	name string
}

// NewBaseConnection creates a new BaseConnection with a unique ID and the provided name.
func NewBaseConnection(name string) BaseConnection {
	return BaseConnection{
		id:   globalConnectionID.Add(1),
		name: name,
	}
}

// ID returns the unique identifier for this connection.
func (b *BaseConnection) ID() int64 {
	return b.id
}
