// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
Package network provides low-level, protocol-specific network connection
implementations (TCP and WebSocket). It is the foundational "socket layer"
of the library, responsible for raw data transmission and framing.

# Architectural Role

This package deals with the raw realities of network programming:
  - Dialing endpoints.
  - Framing messages (via the `Framer` interface).
  - Handling encryption and decryption (via the optional `Cipher` interface).
  - Reading from the socket in a dedicated loop and pushing messages up.

It abstracts these details behind a single `Connection` interface. Higher-level
packages like `steam.transport` use this interface to send and receive logical
Steam messages without needing to know if they are traveling over TCP or WebSockets.

# Connection Lifecycle

 1. A connection is established using a `New...` function (e.g., `NewTCP`, `NewWS`).
    WebSocket connections support optional HTTP headers for the initial handshake.
 2. The function immediately starts a `readLoop` in a background goroutine.
 3. The `readLoop` continuously reads data from the socket. When a full message
    is received, it is pushed into the channel returned by `Messages()`.
 4. Other parts of the application can send data using the `Send` method.
 5. If the connection is terminated (by the remote peer or an error), the `Closed()`
    channel is closed, and any fatal errors are sent to the `Errors()` channel.

This asynchronous model (via Go channels) allows for a clean
separation of network I/O from the business logic of processing messages.

# Error Handling

All functions and methods in the package return structured, custom errors of type `*Error`.
This allows callers to easily programmatically inspect the failed operation, transport type,
and underlying causes:

  - `Op`: Represents the specific operation that failed (e.g. `OpDial`, `OpSend`, `OpRead`, etc.).
  - `Net`: Indicates the transport protocol name (`"TCP"` or `"WS"`).
  - `Err`: The wrapped underlying error (e.g., a standard `net.OpError` or custom protocol failure).

To inspect the error programmatically:

	if err != nil {
		var netErr *network.Error
		if errors.As(err, &netErr) {
			switch netErr.Op {
			case network.OpDial:
				// handle connection failures
			case network.OpSend:
				// handle write failures
			}
		}
	}

Additionally, `network.Error` supports standard Go wrapping. Calling `errors.Is(err, context.Canceled)` or
`errors.Is(err, io.EOF)` on a returned `network.Error` will transparently verify the underlying error.
*/
package network
