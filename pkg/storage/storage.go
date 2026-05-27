// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package storage

import (
	"context"
	"errors"

	"github.com/lemon4ksan/g-man/pkg/steam/auth"
)

// ErrNotFound is returned when a requested key does not exist in the storage.
var ErrNotFound = errors.New("storage: key not found")

// Provider is the master interface that a storage backend must implement.
//
// It acts as a factory for domain-specific stores such as [auth.Store] and [KV].
// Built-in implementations include [jsonfile.Provider] and [memory.Provider].
type Provider interface {
	// Auth returns a store dedicated to authentication data (tokens, machine IDs).
	Auth() auth.Store

	// KV returns a generic key-value store for arbitrary data, isolated by the namespace.
	KV(namespace string) KV

	// Close cleanly shuts down the storage connection and flushes any pending writes.
	Close() error
}

// KV is a generic, string-to-bytes key-value store.
//
// The namespace concept isolates keys between different subsystems,
// preventing collisions between unrelated modules.
type KV interface {
	// Set stores a binary value associated with a key.
	Set(ctx context.Context, key string, value []byte) error

	// Get retrieves a binary value by its key.
	//
	// It returns [ErrNotFound] if the key does not exist.
	Get(ctx context.Context, key string) ([]byte, error)

	// Delete removes a key-value pair from the store.
	Delete(ctx context.Context, key string) error

	// Has reports whether the key exists in the store.
	Has(ctx context.Context, key string) (bool, error)

	// Keys returns all keys in the store that start with the given prefix.
	//
	// If the prefix argument is empty, it returns all keys within the current namespace.
	Keys(ctx context.Context, prefix string) ([]string, error)
}
