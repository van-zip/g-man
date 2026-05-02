// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of a BSD-style
// license that can be found in the LICENSE file.

package steam

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	pb "github.com/lemon4ksan/g-man/pkg/protobuf/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/api"
	"github.com/lemon4ksan/g-man/pkg/steam/auth"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/steam/module"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol/enums"
	"github.com/lemon4ksan/g-man/pkg/steam/socket"
	tr "github.com/lemon4ksan/g-man/pkg/steam/transport"
	"github.com/lemon4ksan/g-man/pkg/storage/memory"
)

func defaultConfig() Config {
	return Config{
		Socket:  socket.DefaultConfig(),
		Storage: memory.New(),
		HTTP:    &http.Client{},
		Device:  &auth.DeviceConfig{},
	}
}

func newTestClient(t *testing.T) (*Client, *mockHTTPDoerImpl) {
	httpDoer := new(mockHTTPDoerImpl)
	cfg := Config{
		Socket:  socket.DefaultConfig(),
		Storage: memory.New(),
		HTTP:    httpDoer,
		Device:  &auth.DeviceConfig{},
	}
	client := NewClient(cfg)
	t.Cleanup(func() { _ = client.Close() })

	return client, httpDoer
}

func TestClient_Initialization(t *testing.T) {
	t.Run("Default Storage", func(t *testing.T) {
		client := NewClient(Config{})
		assert.NotNil(t, client.Storage())
		client.Close()
	})

	t.Run("Module Init and Start Failures", func(t *testing.T) {
		m := new(mockModuleImpl)
		m.On("Name").Return("failing")
		m.On("Init", mock.Anything).Return(errors.New("init fail")).Once()
		m.On("Start", mock.Anything).Return(errors.New("start fail")).Once()
		m.On("Close").Return(nil).Maybe()

		// Functional option to register module BEFORE run() starts
		withMod := func(c *Client) { c.modules["failing"] = m }

		client := NewClient(Config{}, withMod)
		defer client.Close()

		// Allow background goroutine to process
		time.Sleep(50 * time.Millisecond)
		m.AssertExpectations(t)
	})
}

func TestClient_Lifecycle(t *testing.T) {
	client := NewClient(defaultConfig())

	assert.Eventually(t, func() bool { return client.State() == StateRunning }, time.Second, 10*time.Millisecond)
	assert.Equal(t, "running", client.State().String())
	assert.Equal(t, "unknown", State(99).String())

	require.NoError(t, client.Close())
	assert.Equal(t, StateClosed, client.State())

	done := make(chan struct{})
	go func() { client.Wait(); close(done) }()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Wait() did not return")
	}

	err := client.ConnectAndLogin(context.Background(), socket.CMServer{}, nil)
	assert.ErrorIs(t, err, module.ErrClientClosed)
}

func TestClient_Do_TransportSelection(t *testing.T) {
	client, httpDoer := newTestClient(t)
	ctx := context.Background()

	t.Run("Transport Selection Logic", func(t *testing.T) {
		httpDoer.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
		}, nil)

		// 1. Socket target but disconnected -> HTTP
		_, err := client.Do(ctx, tr.NewRequest(&mockSocketTarget{mockTarget{path: "p", method: "GET"}}, nil))
		assert.NoError(t, err)

		// 2. HTTP Target -> HTTP
		_, err = client.Do(ctx, tr.NewRequest(&mockTarget{path: "p", method: "GET"}, nil))
		assert.NoError(t, err)
	})

	t.Run("Silent Refresh Success", func(t *testing.T) {
		// Call 1: Session Expired
		httpDoer.On("Do", mock.MatchedBy(func(r *http.Request) bool {
			return r.URL.Path != "/IAuthenticationService/GenerateAccessTokenForApp/v1"
		})).
			Return(nil, api.ErrSessionExpired).Once()

		// Setup for RefreshSession
		sess := &mockSessionImpl{id: 123}
		client.socket.SetSession(sess)
		client.webSession = &mockWebSession{alive: false}

		// Mock GenerateAccessToken Call
		tokenPb, _ := proto.Marshal(&pb.CAuthentication_AccessToken_GenerateForApp_Response{
			AccessToken: proto.String("new_at"),
		})
		httpDoer.On("Do", mock.MatchedBy(func(r *http.Request) bool {
			return r.URL.Path == "/IAuthenticationService/GenerateAccessTokenForApp/v1"
		})).Return(&http.Response{
			StatusCode: 200,
			Header:     http.Header{"x-eresult": {"1"}},
			Body:       io.NopCloser(bytes.NewBuffer(tokenPb)),
		}, nil).Once()

		// Final Call: Retry Succeeds
		httpDoer.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Body:       io.NopCloser(bytes.NewBufferString(`{}`)),
		}, nil).Once()

		_, err := client.Do(ctx, tr.NewRequest(&mockTarget{path: "p", method: "GET"}, nil))
		assert.NoError(t, err)
	})
}

func TestClient_RefreshSession_Paths(t *testing.T) {
	t.Run("Generate Access Token Fail", func(t *testing.T) {
		client, httpDoer := newTestClient(t)
		client.webSession = &mockWebSession{alive: false}
		client.socket.SetSession(&mockSessionImpl{}) // INJECT SESSION

		httpDoer.On("Do", mock.Anything).Return(nil, errors.New("rpc fail")).Once()

		err := client.RefreshSession(context.Background())
		assert.ErrorContains(t, err, "cannot refresh session")
	})

	t.Run("Web Auth Fail", func(t *testing.T) {
		client, httpDoer := newTestClient(t)
		client.webSession = &mockWebSession{alive: false}
		client.socket.SetSession(&mockSessionImpl{}) // INJECT SESSION

		tokenPb, _ := proto.Marshal(&pb.CAuthentication_AccessToken_GenerateForApp_Response{
			AccessToken: proto.String("t"),
		})
		httpDoer.On("Do", mock.Anything).Return(&http.Response{
			StatusCode: 200,
			Header:     http.Header{"x-eresult": {"1"}},
			Body:       io.NopCloser(bytes.NewBuffer(tokenPb)),
		}, nil).Once()

		err := client.RefreshSession(context.Background())
		assert.ErrorContains(t, err, "cannot refresh session")
	})
}

func TestClient_Shortcuts(t *testing.T) {
	client, _ := newTestClient(t)

	assert.Equal(t, client.socket, client.Socket())
	assert.Equal(t, client.bus, client.Bus())
	assert.Equal(t, client.storage, client.Storage())
	assert.Equal(t, client, client.Service())
	assert.Equal(t, id.ID(0), client.SteamID()) // No session

	// Test handlers
	client.RegisterPacketHandler(enums.EMsg(1), func(p *protocol.Packet) {})
	client.RegisterServiceHandler("A.B", func(p *protocol.Packet) {})
	client.UnregisterPacketHandler(enums.EMsg(1))
	client.UnregisterServiceHandler("A.B")
}

type mockTarget struct {
	path   string
	method string
}

func (m *mockTarget) String() string     { return "mock" }
func (m *mockTarget) HTTPPath() string   { return m.path }
func (m *mockTarget) HTTPMethod() string { return m.method }

type mockSocketTarget struct {
	mockTarget
}

func (m *mockSocketTarget) EMsg(isAuth bool) enums.EMsg { return enums.EMsg(1) }
func (m *mockSocketTarget) ObjectName() string          { return "obj" }

type mockModuleImpl struct{ mock.Mock }

func (m *mockModuleImpl) Name() string                       { return m.Called().String(0) }
func (m *mockModuleImpl) Init(ictx module.InitContext) error { return m.Called(ictx).Error(0) }
func (m *mockModuleImpl) Start(ctx context.Context) error    { return m.Called(ctx).Error(0) }
func (m *mockModuleImpl) StartAuthed(ctx context.Context, actx module.AuthContext) error {
	return m.Called(ctx, actx).Error(0)
}
func (m *mockModuleImpl) Close() error { return m.Called().Error(0) }

type mockHTTPDoerImpl struct{ mock.Mock }

func (m *mockHTTPDoerImpl) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	resp, _ := args.Get(0).(*http.Response)
	return resp, args.Error(1)
}

type mockWebSession struct {
	alive bool
}

func (m *mockWebSession) HTTP() *http.Client                       { return &http.Client{Transport: &mockRoundTripper{}} }
func (m *mockWebSession) SessionID(b string) string                { return "mock_session" }
func (m *mockWebSession) Verify(ctx context.Context) (bool, error) { return m.alive, nil }
func (m *mockWebSession) Authenticate(ctx context.Context, p pb.EAuthTokenPlatformType, r, a string) error {
	return nil
}
func (m *mockWebSession) IsAuthenticated() bool { return true }

type mockRoundTripper struct{}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBuffer(nil))}, nil
}

type mockSessionImpl struct {
	socket.Session
	id           uint64
	refreshToken string
	accessToken  string
}

func (m *mockSessionImpl) SteamID() uint64         { return m.id }
func (m *mockSessionImpl) RefreshToken() string    { return m.refreshToken }
func (m *mockSessionImpl) AccessToken() string     { return m.accessToken }
func (m *mockSessionImpl) SetAccessToken(s string) { m.accessToken = s }
