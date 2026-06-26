// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package router

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/lemon4ksan/g-man/pkg/steam/service"
	tr "github.com/lemon4ksan/g-man/pkg/steam/transport"
)

type testStateProvider struct {
	connected bool
}

func (sp *testStateProvider) IsConnected() bool {
	return sp.connected
}

type testSessionRefresher struct {
	mock.Mock
}

func (m *testSessionRefresher) Refresh(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *testSessionRefresher) Unified() *service.Client {
	args := m.Called()
	client, _ := args.Get(0).(*service.Client)
	return client
}

func (m *testSessionRefresher) Socket() *service.Client {
	args := m.Called()
	client, _ := args.Get(0).(*service.Client)
	return client
}

type testTransport struct {
	mock.Mock
}

func (t *testTransport) Do(ctx context.Context, req *tr.Request) (*tr.Response, error) {
	args := t.Called(ctx, req)
	resp, _ := args.Get(0).(*tr.Response)
	return resp, args.Error(1)
}

type testTarget struct {
	tr.Target
}

type testSocketTarget struct {
	tr.SocketTarget
}

func TestNew(t *testing.T) {
	refresher := &testSessionRefresher{}
	state := &testStateProvider{}

	router := New(refresher, state)
	assert.NotNil(t, router)
	assert.NotNil(t, router.matcher)
	assert.Equal(t, refresher, router.refresher)
	assert.Equal(t, state, router.state)
}

func TestServiceRouter_SetRouteMatcher(t *testing.T) {
	router := New(&testSessionRefresher{}, &testStateProvider{})

	router.SetRouteMatcher(nil)
	assert.NotNil(t, router.matcher)

	customCalled := false
	router.SetRouteMatcher(func(req *tr.Request) TransportType {
		customCalled = true
		return TransportSocket
	})

	req := tr.NewRequest(&testTarget{}, nil)
	assert.Equal(t, TransportSocket, router.matcher(req))
	assert.True(t, customCalled)
}

func TestServiceRouter_DefaultRouteMatcher(t *testing.T) {
	state := &testStateProvider{}
	router := New(&testSessionRefresher{}, state)

	reqSocket := tr.NewRequest(&testSocketTarget{}, nil)
	reqWeb := tr.NewRequest(&testTarget{}, nil)

	t.Run("Socket target, connected", func(t *testing.T) {
		state.connected = true

		assert.Equal(t, TransportSocket, router.DefaultRouteMatcher(reqSocket))
	})

	t.Run("Socket target, disconnected", func(t *testing.T) {
		state.connected = false

		assert.Equal(t, TransportWebAPI, router.DefaultRouteMatcher(reqSocket))
	})

	t.Run("Web target, connected", func(t *testing.T) {
		state.connected = true

		assert.Equal(t, TransportWebAPI, router.DefaultRouteMatcher(reqWeb))
	})

	t.Run("Web target, disconnected", func(t *testing.T) {
		state.connected = false

		assert.Equal(t, TransportWebAPI, router.DefaultRouteMatcher(reqWeb))
	})
}

func TestServiceRouter_Do_TransportSelection(t *testing.T) {
	state := &testStateProvider{}
	unifiedTrans := &testTransport{}
	socketTrans := &testTransport{}

	unified := service.New(unifiedTrans)
	socketAPI := service.New(socketTrans)

	refresher := &testSessionRefresher{}
	refresher.On("Unified").Return(unified)
	refresher.On("Socket").Return(socketAPI)

	router := New(refresher, state)

	t.Run("Fallback to HTTP when socket disconnected", func(t *testing.T) {
		state.connected = false
		req := tr.NewRequest(&testSocketTarget{}, nil)

		unifiedTrans.On("Do", mock.Anything, req).Return((*tr.Response)(nil), nil).Once()

		_, err := router.Do(t.Context(), req)
		assert.NoError(t, err)
		unifiedTrans.AssertExpectations(t)
	})

	t.Run("Route to Socket when connected", func(t *testing.T) {
		state.connected = true
		req := tr.NewRequest(&testSocketTarget{}, nil)

		socketTrans.On("Do", mock.Anything, req).Return((*tr.Response)(nil), nil).Once()

		_, err := router.Do(t.Context(), req)
		assert.NoError(t, err)
		socketTrans.AssertExpectations(t)
	})
}

func TestServiceRouter_Do_SilentRefresh(t *testing.T) {
	state := &testStateProvider{connected: false}
	unifiedTrans := &testTransport{}
	unified := service.New(unifiedTrans)

	refresher := &testSessionRefresher{}
	refresher.On("Unified").Return(unified)

	router := New(refresher, state)
	req := tr.NewRequest(&testTarget{}, nil)

	t.Run("Silent Refresh - Success on second attempt", func(t *testing.T) {
		unifiedTrans.On("Do", mock.Anything, req).Return((*tr.Response)(nil), service.ErrSessionExpired).Once()

		refresher.On("Refresh", t.Context()).Return(nil).Once()

		unifiedTrans.On("Do", mock.Anything, req).Return((*tr.Response)(nil), nil).Once()

		_, err := router.Do(t.Context(), req)
		assert.NoError(t, err)
		unifiedTrans.AssertExpectations(t)
		refresher.AssertExpectations(t)
	})

	t.Run("Silent Refresh - Refresh Fails", func(t *testing.T) {
		unifiedTrans.On("Do", mock.Anything, req).Return((*tr.Response)(nil), service.ErrSessionExpired).Once()

		refresher.On("Refresh", t.Context()).Return(errors.New("refresh failed")).Once()

		_, err := router.Do(t.Context(), req)
		assert.ErrorContains(t, err, "router: auto-refresh failed: refresh failed")
		unifiedTrans.AssertExpectations(t)
		refresher.AssertExpectations(t)
	})
}

func TestServiceRouter_Do_GenericError(t *testing.T) {
	state := &testStateProvider{connected: false}
	unifiedTrans := &testTransport{}
	unified := service.New(unifiedTrans)

	refresher := &testSessionRefresher{}
	refresher.On("Unified").Return(unified)

	router := New(refresher, state)
	req := tr.NewRequest(&testTarget{}, nil)

	unifiedTrans.On("Do", mock.Anything, req).Return((*tr.Response)(nil), errors.New("network error")).Once()

	_, err := router.Do(t.Context(), req)
	assert.ErrorContains(t, err, "network error")
	unifiedTrans.AssertExpectations(t)
}

func TestServiceRouter_Do_NilClient(t *testing.T) {
	state := &testStateProvider{connected: true}
	refresher := &testSessionRefresher{}
	router := New(refresher, state)

	t.Run("Socket client is nil", func(t *testing.T) {
		refresher.On("Socket").Return((*service.Client)(nil)).Once()

		req := tr.NewRequest(&testSocketTarget{}, nil)

		_, err := router.Do(t.Context(), req)
		assert.ErrorIs(t, err, ErrNoActiveClient)
	})

	t.Run("WebAPI client is nil", func(t *testing.T) {
		state.connected = false

		refresher.On("Unified").Return((*service.Client)(nil)).Once()

		req := tr.NewRequest(&testTarget{}, nil)

		_, err := router.Do(t.Context(), req)
		assert.ErrorIs(t, err, ErrNoActiveClient)
	})
}
