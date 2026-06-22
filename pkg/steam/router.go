// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package steam

import (
	"context"
	"errors"
	"fmt"

	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	"github.com/lemon4ksan/g-man/pkg/steam/service"
	tr "github.com/lemon4ksan/g-man/pkg/steam/transport"
)

// SessionRefresher is the interface for refreshing the session.
type SessionRefresher interface {
	// Refresh refreshes the session using the provided context.
	Refresh(ctx context.Context) error
	// Clients returns the unified and socket API clients.
	Clients() (unified, socketAPI *service.Client)
}

// TransportType represents the selected transport channel.
type TransportType int

const (
	// TransportWebAPI routes the request over HTTPS WebAPI.
	TransportWebAPI TransportType = iota
	// TransportSocket routes the request over the active socket connection.
	TransportSocket
)

// RouteMatcher determines the target transport for a request.
type RouteMatcher func(req *tr.Request, socketConnected bool) TransportType

// ServiceRouter encapsulates transport selection and automatic retries logic.
//
// It routes requests over TCP/WebSockets or HTTP based on connectivity state.
// Use [NewServiceRouter] to create new instances of the router.
type ServiceRouter struct {
	session SessionRefresher
	socket  SocketProvider
	matcher RouteMatcher
}

// NewServiceRouter creates a new service router.
func NewServiceRouter(sess SessionRefresher, sock SocketProvider) *ServiceRouter {
	return &ServiceRouter{
		session: sess,
		socket:  sock,
		matcher: DefaultRouteMatcher,
	}
}

// SetRouteMatcher overrides the default transport selection logic.
func (r *ServiceRouter) SetRouteMatcher(matcher RouteMatcher) {
	if matcher == nil {
		r.matcher = DefaultRouteMatcher
	} else {
		r.matcher = matcher
	}
}

// DefaultRouteMatcher implements the standard transport selection logic.
func DefaultRouteMatcher(req *tr.Request, socketConnected bool) TransportType {
	_, isSocketCompatible := req.Target().(tr.SocketTarget)
	if socketConnected && isSocketCompatible {
		return TransportSocket
	}

	return TransportWebAPI
}

// Do performs the request, automatically choosing between SocketProvider and HTTP,
// and handles transparent session updates if necessary.
func (r *ServiceRouter) Do(ctx context.Context, req *tr.Request) (*tr.Response, error) {
	resp, err := r.perform(ctx, req)

	if err != nil && errors.Is(err, service.ErrSessionExpired) {
		if refreshErr := r.session.Refresh(ctx); refreshErr != nil {
			return nil, fmt.Errorf("router: auto-refresh failed: %w", refreshErr)
		}

		return r.perform(ctx, req)
	}

	return resp, err
}

func (r *ServiceRouter) perform(ctx context.Context, req *tr.Request) (*tr.Response, error) {
	uClient, sClient := r.session.Clients()

	var selected service.Doer

	transport := r.matcher(req, r.socket.IsConnected())
	switch transport {
	case TransportSocket:
		selected = sClient
	case TransportWebAPI:
		selected = uClient
		ctx = protocol.WithTransportType(ctx, protocol.TransportWebAPI)
	}

	if selected == nil {
		return nil, errors.New("router: no active client for target transport")
	}

	return selected.Do(ctx, req)
}
