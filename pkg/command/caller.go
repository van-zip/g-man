// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package command

import "context"

type contextKey string

// CallerKey and TransportKey are context keys used to store caller and transport information.
const (
	CallerKey    contextKey = "cmd_caller"
	TransportKey contextKey = "cmd_transport"
)

// Caller abstracts the identity executing the command.
type Caller interface {
	// ID returns a unique string representation of the identifier (such as SteamID, Discord ID, or "console").
	ID() string
	// DisplayName returns a human-readable display name of the caller.
	DisplayName() string
	// IsAdmin reports whether the caller has administrator/privilege execution rights.
	IsAdmin() bool
}

// WithCaller injects Caller into context.
func WithCaller(ctx context.Context, caller Caller) context.Context {
	return context.WithValue(ctx, CallerKey, caller)
}

// CallerFromContext extracts Caller from context.
func CallerFromContext(ctx context.Context) (Caller, bool) {
	c, ok := ctx.Value(CallerKey).(Caller)
	return c, ok
}

// WithTransport injects Transport name into context.
func WithTransport(ctx context.Context, transport string) context.Context {
	return context.WithValue(ctx, TransportKey, transport)
}

// TransportFromContext extracts Transport name from context.
func TransportFromContext(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(TransportKey).(string)
	return t, ok
}
