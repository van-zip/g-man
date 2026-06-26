// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package steam

import "github.com/lemon4ksan/g-man/pkg/steam/client"

// ErrNotRunning is returned when the client is not running.
var ErrNotRunning = client.ErrNotRunning

// ErrSocketDisabled is returned when attempting socket operations while the transport layer is disabled.
var ErrSocketDisabled = client.ErrSocketDisabled

// Client acts as the central hub connecting socket, authentication, and modules.
// Create new instances of Client using [NewClient] or [NewReadyClient].
type Client = client.Client

// Config aggregates configurations for all core subsystems of [Client].
// Use [DefaultConfig] to initialize a configuration with standard settings.
type Config = client.Config

// DefaultConfig returns the baseline [Config] with standard defaults.
var DefaultConfig = client.DefaultConfig

// Option defines a functional configuration option for [Client].
type Option = client.Option

// WithLogger sets a custom [log.Logger] for [Client].
var WithLogger = client.WithLogger

// WithModule adds a [module.Module] to [Client] and initializes it immediately.
var WithModule = client.WithModule

// WithSocket sets a custom [SocketProvider] for [Client].
var WithSocket = client.WithSocket

// WithREST sets a custom [aoni.Client] for [Client].
var WithREST = client.WithREST

// WithBus sets a custom [bus.Bus] for [Client].
var WithBus = client.WithBus

// WithStorage sets a custom [storage.Provider] for [Client].
var WithStorage = client.WithStorage

// WithAuthenticator sets a custom [AuthenticatorProvider] for [Client].
var WithAuthenticator = client.WithAuthenticator

// WithWebFactory sets a custom [WebSessionFactory] for [Client].
var WithWebFactory = client.WithWebFactory

// WithCommunityFactory sets a custom [CommunityClientFactory] for [Client].
var WithCommunityFactory = client.WithCommunityFactory

// NewClient initializes and returns a new [Client] with the given [Config] and [Option] list.
// Returns an error if option application fails or configuration is invalid.
var NewClient = client.New

// NewReadyClient creates a [Client], configures a default logger if none is provided, connects to the optimal server, and performs logon.
// It returns an error if CM server discovery fails, connection fails, or login is rejected.
// It returns an error if the context ctx is canceled or details is nil.
var NewReadyClient = client.NewReady

// GetModule returns the first registered module matching type T from the [Client].
// Returns the zero value of T if no matching module is registered.
// Returns the zero value of T if c is nil.
func GetModule[T any](c *client.Client) T {
	return client.GetModule[T](c)
}
