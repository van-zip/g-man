// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/miyako/bus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	"github.com/lemon4ksan/g-man/pkg/log"
	pb "github.com/lemon4ksan/g-man/pkg/protobuf/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/auth"
	"github.com/lemon4ksan/g-man/pkg/steam/client/router"
	"github.com/lemon4ksan/g-man/pkg/steam/client/session"
	"github.com/lemon4ksan/g-man/pkg/steam/community"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/steam/module"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol/enums"
	"github.com/lemon4ksan/g-man/pkg/steam/socket"
	tr "github.com/lemon4ksan/g-man/pkg/steam/transport"
	"github.com/lemon4ksan/g-man/pkg/storage"
)

type mockAuthenticator struct {
	mock.Mock
}

func (m *mockAuthenticator) LogOn(ctx context.Context, details *auth.LogOnDetails, server socket.CMServer) error {
	args := m.Called(ctx, details, server)
	return args.Error(0)
}

type mockWebSession struct {
	mock.Mock
}

func (m *mockWebSession) HTTP() *http.Client {
	args := m.Called()
	return args.Get(0).(*http.Client)
}

func (m *mockWebSession) SessionID(baseURL string) string {
	args := m.Called(baseURL)
	return args.String(0)
}

func (m *mockWebSession) Verify(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

func (m *mockWebSession) Authenticate(
	ctx context.Context,
	platformType pb.EAuthTokenPlatformType,
	refreshToken, accessToken string,
) error {
	args := m.Called(ctx, platformType, refreshToken, accessToken)
	return args.Error(0)
}

func (m *mockWebSession) IsAuthenticated() bool {
	args := m.Called()
	return args.Bool(0)
}

type mockCommunity struct {
	community.Requester
	mock.Mock
}

func (m *mockCommunity) GetOrRegisterAPIKey(ctx context.Context, domain string) (string, error) {
	args := m.Called(ctx, domain)
	return args.String(0), args.Error(1)
}

type mockSocket struct {
	session.SocketProvider
	mock.Mock
}

func (m *mockSocket) IsConnected() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *mockSocket) UpdateLogger(logger log.Logger) {
	m.Called(logger)
}

func (m *mockSocket) UpdateServers(servers []socket.CMServer) {
	m.Called(servers)
}

func (m *mockSocket) Send(ctx context.Context, build socket.PayloadBuilder, opts ...socket.SendOption) error {
	args := m.Called(ctx, build, opts)
	return args.Error(0)
}

func (m *mockSocket) RegisterMsgHandler(eMsg enums.EMsg, handler socket.Handler) {
	m.Called(eMsg, handler)
}

func (m *mockSocket) RegisterServiceHandler(method string, handler socket.Handler) {
	m.Called(method, handler)
}

func (m *mockSocket) Disconnect() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockSocket) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockSocket) Session() socket.Session {
	args := m.Called()
	sess, _ := args.Get(0).(socket.Session)
	return sess
}

func (m *mockSocket) SendProto(
	ctx context.Context,
	eMsg enums.EMsg,
	req proto.Message,
	opts ...socket.SendOption,
) error {
	args := m.Called(ctx, eMsg, req, opts)
	return args.Error(0)
}

type mockSession struct {
	socket.Session
	mock.Mock
}

func (m *mockSession) SteamID() uint64 {
	args := m.Called()
	return args.Get(0).(uint64)
}

func (m *mockSession) AccessToken() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockSession) RefreshToken() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockSession) SetAccessToken(token string) {
	m.Called(token)
}

type mockHTTPDoer struct {
	mock.Mock
}

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	resp, _ := args.Get(0).(*http.Response)
	return resp, args.Error(1)
}

func (m *mockHTTPDoer) Request(
	ctx context.Context,
	method, url string,
	opts ...aoni.RequestModifier,
) (*http.Response, error) {
	args := m.Called(ctx, method, url, opts)
	resp, _ := args.Get(0).(*http.Response)
	return resp, args.Error(1)
}

type mockModule struct {
	mock.Mock
}

func (m *mockModule) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *mockModule) Init(ctx module.InitContext) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockModule) Start(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

type mockAuthModule struct {
	mockModule
}

func (m *mockAuthModule) StartAuthed(ctx context.Context, authCtx module.AuthContext) error {
	args := m.Called(ctx, authCtx)
	return args.Error(0)
}

type mockTarget struct {
	tr.Target
}

func (mockTarget) HTTPPath() string   { return "/test" }
func (mockTarget) HTTPMethod() string { return "GET" }

type testMocks struct {
	auth *mockAuthenticator
	web  *mockWebSession
	comm *mockCommunity
	sock *mockSocket
	http *mockHTTPDoer
}

func setupTestClient(t *testing.T) (*Client, *testMocks) {
	m := &testMocks{
		auth: new(mockAuthenticator),
		web:  new(mockWebSession),
		comm: new(mockCommunity),
		sock: new(mockSocket),
		http: new(mockHTTPDoer),
	}

	opts := []Option{
		WithSocket(m.sock),
		WithAuthenticator(m.auth),
		WithWebFactory(func(steamID id.ID, logger log.Logger, baseDoer aoni.HTTPDoer) session.WebSessionProvider {
			return m.web
		}),
		WithCommunityFactory(
			func(httpClient *http.Client, sessionID func(string) string, logger log.Logger) session.CommunityProvider {
				return m.comm
			},
		),
	}

	c, err := New(Config{}, opts...)
	require.NoError(t, err)

	c.rest = aoni.NewClient(m.http)
	c.socket = m.sock
	c.session = session.New(m.sock, session.Config{
		HTTP:          m.http,
		Authenticator: m.auth,
		WebFactory: func(steamID id.ID, logger log.Logger, baseDoer aoni.HTTPDoer) session.WebSessionProvider {
			return m.web
		},
		CommunityFactory: func(httpClient *http.Client, sessionID func(string) string, logger log.Logger) session.CommunityProvider {
			return m.comm
		},
	})
	c.router = router.New(c.session, m.sock)

	m.sock.On("Close").Return(nil).Maybe()
	m.sock.On("UpdateLogger", mock.Anything).Return().Maybe()
	m.sock.On("UpdateServers", mock.Anything).Return().Maybe()
	m.web.On("Verify", mock.Anything).Return(true, nil).Maybe()
	m.web.On("HTTP").Return(&http.Client{}).Maybe()
	m.comm.On("GetOrRegisterAPIKey", mock.Anything, mock.Anything).Return("key_123", nil).Maybe()

	return c, m
}

func TestGetModule(t *testing.T) {
	t.Run("c is nil", func(t *testing.T) {
		res := GetModule[*mockAuthModule](nil)
		assert.Nil(t, res)
	})

	t.Run("module found", func(t *testing.T) {
		mod := &mockAuthModule{}
		mod.On("Name").Return("auth")

		c, _ := New(Config{DisableSocket: true}, WithModule(mod))
		res := GetModule[*mockAuthModule](c)
		assert.Equal(t, mod, res)
		c.Close()
	})

	t.Run("module not found", func(t *testing.T) {
		mod := &mockModule{}
		mod.On("Name").Return("simple")

		c, _ := New(Config{DisableSocket: true}, WithModule(mod))
		res := GetModule[*mockAuthModule](c)
		assert.Nil(t, res)
		c.Close()
	})
}

func TestConfig_ResolveDefaults(t *testing.T) {
	t.Run("ProxyURL copied", func(t *testing.T) {
		cfg := Config{
			ProxyURL: "http://my-proxy",
		}
		cfg.ResolveDefaults()
		assert.Equal(t, "http://my-proxy", cfg.Socket.Connector.ProxyURL)
	})

	t.Run("ProxyURL not overwritten", func(t *testing.T) {
		cfg := Config{
			ProxyURL: "http://my-proxy",
		}
		cfg.Socket.Connector.ProxyURL = "http://socket-proxy"
		cfg.ResolveDefaults()
		assert.Equal(t, "http://socket-proxy", cfg.Socket.Connector.ProxyURL)
	})
}

func TestClient_LifecycleState(t *testing.T) {
	client, _ := New(Config{})

	_ = client.Run()

	assert.Equal(t, StateRunning, client.State())
	assert.Equal(t, "running", client.State().String())

	client.Close()
	client.Close()

	assert.Equal(t, StateClosed, client.State())
	assert.ErrorIs(t, client.ConnectAndLogin(context.Background(), socket.CMServer{}, nil), module.ErrClosed)
}

func TestClient_Initialization(t *testing.T) {
	assert.NotNil(t, DefaultConfig().Socket)

	t.Run("Default Storage Assignment", func(t *testing.T) {
		client, _ := New(Config{DisableSocket: true})
		assert.NotNil(t, client.Storage())
		client.Close()
	})

	t.Run("Options", func(t *testing.T) {
		l := log.Discard
		mod := new(mockModule)
		mod.On("Name").Return("opt_mod")
		mod.On("Init", mock.Anything).Return(nil).Once()
		mod.On("Start", mock.Anything).Return(nil).Once()

		client, err := New(Config{DisableSocket: true}, WithLogger(l), WithModule(mod))
		assert.NoError(t, err)
		assert.Equal(t, client.Logger(), l)
		assert.NotNil(t, client.Module("opt_mod"))
		client.Close()
	})
}

func TestClient_Options_Extra(t *testing.T) {
	restClient := aoni.NewClient(nil)
	eventBus := bus.New()
	storageProv := storage.Provider(nil)
	authProv := &mockAuthenticator{}

	webFact := func(steamID id.ID, logger log.Logger, baseDoer aoni.HTTPDoer) session.WebSessionProvider {
		return nil
	}
	commFact := func(httpClient *http.Client, sessionID func(string) string, logger log.Logger) session.CommunityProvider {
		return nil
	}

	c, err := New(Config{DisableSocket: true},
		WithREST(restClient),
		WithBus(eventBus),
		WithStorage(storageProv),
		WithAuthenticator(authProv),
		WithWebFactory(webFact),
		WithCommunityFactory(commFact),
	)
	assert.NoError(t, err)

	defer c.Close()

	assert.Equal(t, restClient, c.rest)
	assert.Equal(t, eventBus, c.bus)
	assert.Equal(t, storageProv, c.storage)
	assert.Equal(t, authProv, c.authenticator)
}

func TestClient_RunFailures(t *testing.T) {
	t.Run("Init Fails", func(t *testing.T) {
		mod := new(mockModule)
		mod.On("Name").Return("bad_init")
		mod.On("Init", mock.Anything).Return(errors.New("init fail")).Once()

		client, err := New(Config{}, WithModule(mod))
		assert.NoError(t, err)
		assert.NotNil(t, client)

		err = client.Run()
		assert.ErrorContains(t, err, "init fail")
	})

	t.Run("Start Fails", func(t *testing.T) {
		mod := new(mockModule)
		mod.On("Name").Return("bad_start")
		mod.On("Init", mock.Anything).Return(nil).Once()
		mod.On("Start", mock.Anything).Return(errors.New("start fail")).Once()

		client, err := New(Config{}, WithModule(mod))
		assert.NoError(t, err)
		assert.NotNil(t, client)

		err = client.Run()
		assert.ErrorContains(t, err, "start fail")
	})
}

func TestClient_StateString(t *testing.T) {
	assert.Equal(t, "new", StateNew.String())
	assert.Equal(t, "running", StateRunning.String())
	assert.Equal(t, "closed", StateClosed.String())
	assert.Equal(t, "unknown", State(999).String())
}

func TestClient_Getters(t *testing.T) {
	c, m := setupTestClient(t)
	defer c.Close()

	assert.Equal(t, StateNew, c.State())
	assert.False(t, c.IsAuthorized())
	assert.False(t, c.IsRunning())

	assert.Equal(t, c.session, c.Session())
	assert.Equal(t, c.router, c.Router())
	assert.Equal(t, m.sock, c.Socket())
	assert.NotNil(t, c.Bus())
	assert.NotNil(t, c.Logger())
	assert.NotNil(t, c.Rest())

	c.Close()
	c.Wait()
}

func TestClient_SteamID(t *testing.T) {
	c, m := setupTestClient(t)

	t.Run("Session exists", func(t *testing.T) {
		msess := new(mockSession)
		msess.On("SteamID").Return(uint64(123456))
		m.sock.On("Session").Return(msess).Once()
		assert.Equal(t, uint64(123456), c.Session().SteamID().Uint64())
	})

	t.Run("No session", func(t *testing.T) {
		m.sock.On("Session").Return(nil).Once()
		assert.Equal(t, uint64(0), c.Session().SteamID().Uint64())
	})
}

func TestClient_Do_State(t *testing.T) {
	c, _ := setupTestClient(t)
	c.fsm.ForceSet(StateClosed)

	_, err := c.Do(context.Background(), tr.NewRequest(&mockTarget{}, nil))
	assert.ErrorIs(t, err, ErrNotRunning)
}

func TestClient_Do(t *testing.T) {
	c, m := setupTestClient(t)
	defer c.Close()

	req := tr.NewRequest(&mockTarget{}, nil)

	t.Run("Not running", func(t *testing.T) {
		_, err := c.Do(context.Background(), req)
		assert.ErrorIs(t, err, ErrNotRunning)
	})

	t.Run("Running", func(t *testing.T) {
		c.fsm.ForceSet(StateRunning)

		m.sock.On("IsConnected").Return(false)
		m.http.On("Do", mock.Anything).Return(&http.Response{StatusCode: 200}, nil).Once()

		resp, err := c.Do(context.Background(), req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

func TestClient_RegisterModule(t *testing.T) {
	c, _ := setupTestClient(t)
	defer c.Close()

	c.RegisterModule(nil)

	mod := &mockModule{}
	mod.On("Name").Return("mod-dup")
	mod.On("Init", mock.Anything).Return(nil).Once()
	mod.On("Start", mock.Anything).Return(nil).Once()

	c.RegisterModule(mod)
	c.RegisterModule(mod)
}

func TestClient_DisableSocket(t *testing.T) {
	cfg := Config{
		DisableSocket: true,
	}
	c, err := New(cfg)
	assert.NoError(t, err)

	defer c.Close()

	assert.IsType(t, noopSocketProvider{}, c.Socket())

	err = c.ConnectAndLogin(context.Background(), socket.CMServer{}, &auth.LogOnDetails{})
	assert.ErrorIs(t, err, ErrSocketDisabled)
}

func TestClient_SetPersonaState(t *testing.T) {
	c, m := setupTestClient(t)
	defer c.Close()

	ctx := context.Background()

	m.sock.On("SendProto", ctx, enums.EMsg_ClientChangeStatus, mock.Anything, mock.Anything).Return(nil).Once()

	err := c.SetPersonaState(ctx, enums.EPersonaState_Online)
	assert.NoError(t, err)
	assert.Equal(t, enums.EPersonaState_Online, c.getPersonaState())
}

func TestClient_ConnectAndLogin_Failures(t *testing.T) {
	c, m := setupTestClient(t)
	server := socket.CMServer{}
	details := &auth.LogOnDetails{}

	t.Run("Already Closed", func(t *testing.T) {
		c.fsm.ForceSet(StateClosed)
		err := c.ConnectAndLogin(context.Background(), server, details)
		assert.ErrorIs(t, err, module.ErrClosed)
	})

	c.fsm.ForceSet(StateRunning)

	t.Run("LogOn Fails", func(t *testing.T) {
		m.auth.On("LogOn", mock.Anything, details, server).Return(errors.New("logon fail")).Once()
		err := c.ConnectAndLogin(context.Background(), server, details)
		assert.ErrorContains(t, err, "logon fail")
	})

	t.Run("StartAuthedAll Fails", func(t *testing.T) {
		m.auth.On("LogOn", mock.Anything, details, server).Return(nil).Once()
		m.web.On("Verify", mock.Anything).Return(true, nil)
		m.comm.On("GetOrRegisterAPIKey", mock.Anything, mock.Anything).Return("key_123", nil)
		m.sock.On("SendProto", mock.Anything, enums.EMsg_ClientChangeStatus, mock.Anything, mock.Anything).
			Return(nil).
			Maybe()
		m.sock.On("Session").Return(nil)

		mod := new(mockAuthModule)
		mod.On("Name").Return("auth_mod")
		mod.On("Init", mock.Anything).Return(nil).Once()
		mod.On("Start", mock.Anything).Return(nil).Once()
		mod.On("StartAuthed", mock.Anything, mock.Anything).Return(errors.New("start authed fail")).Once()

		c.RegisterModule(mod)

		err := c.ConnectAndLogin(context.Background(), server, details)
		assert.ErrorContains(t, err, "start authed fail")
	})
}

func TestClient_ConnectAndLogin_EdgeCases(t *testing.T) {
	c, m := setupTestClient(t)
	defer c.Close()

	ctx := context.Background()
	server := socket.CMServer{Endpoint: "cm.test"}
	details := &auth.LogOnDetails{AccountName: "acc", SteamID: 123}

	t.Run("details is nil", func(t *testing.T) {
		err := c.ConnectAndLogin(ctx, server, nil)
		assert.ErrorIs(t, err, ErrNilLogOnDetails)
	})

	t.Run("SetPersonaState fails", func(t *testing.T) {
		c.fsm.ForceSet(StateRunning)
		m.auth.On("LogOn", ctx, details, server).Return(nil).Once()
		m.web.On("Verify", mock.Anything).Return(true, nil).Once()
		m.sock.On("Session").Return(nil)
		m.sock.On("SendProto", ctx, enums.EMsg_ClientChangeStatus, mock.Anything, mock.Anything).
			Return(errors.New("proto err")).
			Once()

		err := c.ConnectAndLogin(ctx, server, details)
		assert.NoError(t, err)
		assert.Equal(t, StateAuthorized, c.State())
	})
}

func TestClient_Reconnect_OptimalCMDiscovery_Success(t *testing.T) {
	c, m := setupTestClient(t)
	defer c.Close()

	ctx := context.Background()

	m.sock.On("Disconnect").Return(nil).Once()

	m.http.On("Do", mock.MatchedBy(func(r *http.Request) bool {
		return r.URL.Path == "/ISteamDirectory/GetCMListForConnect/v1" ||
			r.URL.Path == "/ISteamDirectory/GetCMListForConnect/v1/" ||
			r.URL.Path == "/ISteamDirectory/GetCMList/v1" ||
			r.URL.Path == "/ISteamDirectory/GetCMList/v1/"
	})).Return(&http.Response{
		StatusCode: 200,
		Body: io.NopCloser(
			bytes.NewBufferString(`{"response":{"serverlist":["cm1.steampowered.com:27017"],"success":true}}`),
		),
	}, nil).Once()

	details := &auth.LogOnDetails{AccountName: "acc", SteamID: 123}

	c.session.SetLogonServer(socket.CMServer{Endpoint: "stored.cm"})

	m.auth.On("LogOn", ctx, details, mock.Anything).Return(nil)
	m.sock.On("SendProto", ctx, enums.EMsg_ClientChangeStatus, mock.Anything, mock.Anything).Return(nil)

	err := c.ConnectAndLogin(ctx, socket.CMServer{Endpoint: "stored.cm"}, details)
	assert.NoError(t, err)

	m.sock.On("Disconnect").Return(nil).Once()

	err = c.Reconnect(ctx)
	assert.NoError(t, err)
}

func TestClient_Reconnect_OptimalCMDiscovery_Failure(t *testing.T) {
	c, m := setupTestClient(t)
	defer c.Close()

	ctx := context.Background()

	details := &auth.LogOnDetails{AccountName: "acc", SteamID: 123}
	m.auth.On("LogOn", ctx, details, mock.Anything).Return(nil)
	m.sock.On("SendProto", ctx, enums.EMsg_ClientChangeStatus, mock.Anything, mock.Anything).Return(nil)

	err := c.ConnectAndLogin(ctx, socket.CMServer{Endpoint: "stored.cm"}, details)
	assert.NoError(t, err)

	m.sock.On("Disconnect").Return(errors.New("disc err")).Once()
	m.http.On("Do", mock.Anything).Return(nil, errors.New("http err")).Once()

	err = c.Reconnect(ctx)
	assert.NoError(t, err)
}

func TestClient_Reconnect_ReconnectFailure(t *testing.T) {
	c, m := setupTestClient(t)
	defer c.Close()

	ctx := context.Background()

	details := &auth.LogOnDetails{AccountName: "acc", SteamID: 123}
	m.auth.On("LogOn", ctx, details, mock.Anything).Return(nil).Once()
	m.sock.On("SendProto", ctx, enums.EMsg_ClientChangeStatus, mock.Anything, mock.Anything).Return(nil)

	err := c.ConnectAndLogin(ctx, socket.CMServer{Endpoint: "stored.cm"}, details)
	assert.NoError(t, err)

	m.sock.On("Disconnect").Return(nil).Once()
	m.http.On("Do", mock.Anything).Return(nil, errors.New("http err")).Once()

	m.auth.On("LogOn", ctx, details, mock.Anything).Return(errors.New("logon fail")).Once()

	err = c.Reconnect(ctx)
	assert.ErrorContains(t, err, "reconnect failed")
}

func TestClient_Reconnect_Closed(t *testing.T) {
	c, _ := setupTestClient(t)
	c.fsm.ForceSet(StateClosed)
	err := c.Reconnect(context.Background())
	assert.ErrorIs(t, err, module.ErrClosed)
}

func TestClient_Disconnect(t *testing.T) {
	c, m := setupTestClient(t)
	defer c.Close()

	m.sock.On("Disconnect").Return(errors.New("disc err")).Once()

	err := c.Disconnect()
	assert.ErrorContains(t, err, "disc err")
}

func TestNewReady_Success(t *testing.T) {
	ctx := context.Background()

	m := &testMocks{
		auth: new(mockAuthenticator),
		web:  new(mockWebSession),
		comm: new(mockCommunity),
		sock: new(mockSocket),
		http: new(mockHTTPDoer),
	}

	opts := []Option{
		WithSocket(m.sock),
		WithAuthenticator(m.auth),
		WithREST(aoni.NewClient(m.http)),
		WithWebFactory(func(steamID id.ID, logger log.Logger, baseDoer aoni.HTTPDoer) session.WebSessionProvider {
			return m.web
		}),
		WithCommunityFactory(
			func(httpClient *http.Client, sessionID func(string) string, logger log.Logger) session.CommunityProvider {
				return m.comm
			},
		),
	}

	details := &auth.LogOnDetails{AccountName: "acc", SteamID: 123}

	m.auth.On("LogOn", mock.Anything, details, mock.Anything).Return(nil).Once()
	m.web.On("Verify", mock.Anything).Return(true, nil)
	m.web.On("HTTP").Return(&http.Client{}).Maybe()
	m.comm.On("GetOrRegisterAPIKey", mock.Anything, mock.Anything).Return("key_123", nil).Once()

	m.http.On("Do", mock.MatchedBy(func(r *http.Request) bool {
		return r.URL.Path == "/ISteamDirectory/GetCMListForConnect/v1" ||
			r.URL.Path == "/ISteamDirectory/GetCMListForConnect/v1/" ||
			r.URL.Path == "/ISteamDirectory/GetCMList/v1" ||
			r.URL.Path == "/ISteamDirectory/GetCMList/v1/"
	})).Return(&http.Response{
		StatusCode: 200,
		Body: io.NopCloser(
			bytes.NewBufferString(
				`{"response":{"serverlist":[{"endpoint": "cm1.steampowered.com:27017"}],"success":true}}`,
			),
		),
	}, nil).Once()

	m.sock.On("SendProto", mock.Anything, enums.EMsg_ClientChangeStatus, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	m.sock.On("Disconnect").Return(nil).Once()
	m.sock.On("Close").Return(nil).Once()

	c, err := NewReady(ctx, Config{}, details, opts...)
	assert.NoError(t, err)
	assert.NotNil(t, c)

	err = c.Close()
	assert.NoError(t, err)
}

func TestNewReady_DirectoryFailure(t *testing.T) {
	ctx := context.Background()

	m := &testMocks{
		sock: new(mockSocket),
		http: new(mockHTTPDoer),
	}

	opts := []Option{
		WithSocket(m.sock),
		WithREST(aoni.NewClient(m.http)),
	}

	details := &auth.LogOnDetails{AccountName: "acc", SteamID: 123}

	m.http.On("Do", mock.Anything).Return(nil, errors.New("http err")).Once()

	c, err := NewReady(ctx, Config{}, details, opts...)
	assert.ErrorContains(t, err, "http err")
	assert.Nil(t, c)
}

func TestNoopSocketProvider(t *testing.T) {
	p := noopSocketProvider{}
	ctx := context.Background()

	assert.False(t, p.IsConnected())
	assert.Nil(t, p.Session())
	assert.ErrorIs(t, p.Connect(ctx, socket.CMServer{}), ErrSocketDisabled)
	assert.ErrorIs(t, p.LogOn(ctx, nil), ErrSocketDisabled)
	assert.False(t, p.SetEncryptionKey(nil))
	assert.ErrorIs(t, p.Send(ctx, nil), ErrSocketDisabled)

	pkt, err := p.SendSync(ctx, nil)
	assert.Nil(t, pkt)
	assert.ErrorIs(t, err, ErrSocketDisabled)

	assert.ErrorIs(t, p.SendProto(ctx, enums.EMsg_Invalid, nil), ErrSocketDisabled)
	assert.ErrorIs(t, p.SendRaw(ctx, enums.EMsg_Invalid, nil), ErrSocketDisabled)

	p.RegisterMsgHandler(enums.EMsg_Invalid, nil)
	p.RegisterServiceHandler("", nil)

	assert.ErrorIs(t, p.StartHeartbeat(0), ErrSocketDisabled)
	assert.NoError(t, p.Disconnect())
	assert.NoError(t, p.Close())
	p.UpdateLogger(log.Discard)
	p.UpdateServers(nil)
}

func TestInitContext(t *testing.T) {
	c, m := setupTestClient(t)
	defer c.Close()

	ctx := &initContext{client: c}

	assert.Equal(t, c.storage, ctx.Storage())
	assert.Equal(t, c.bus, ctx.Bus())
	assert.Equal(t, c.Logger(), ctx.Logger())
	assert.Equal(t, c, ctx.Service())
	assert.Equal(t, c.rest, ctx.Rest())
	assert.Equal(t, c.Module("test"), ctx.Module("test"))

	m.sock.On("RegisterMsgHandler", enums.EMsg_ClientLogOnResponse, mock.Anything).Return().Times(2)
	ctx.RegisterPacketHandler(enums.EMsg_ClientLogOnResponse, func(p *protocol.Packet) {})
	ctx.UnregisterPacketHandler(enums.EMsg_ClientLogOnResponse)

	m.sock.On("RegisterServiceHandler", "method", mock.Anything).Return().Times(2)
	ctx.RegisterServiceHandler("method", func(p *protocol.Packet) {})
	ctx.UnregisterServiceHandler("method")

	m.sock.AssertExpectations(t)
}
