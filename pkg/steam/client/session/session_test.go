// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package session

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/lemon4ksan/aoni"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/protobuf/proto"

	"github.com/lemon4ksan/g-man/pkg/log"
	pb "github.com/lemon4ksan/g-man/pkg/protobuf/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/auth"
	"github.com/lemon4ksan/g-man/pkg/steam/community"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/steam/module"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol/enums"
	"github.com/lemon4ksan/g-man/pkg/steam/service"
	"github.com/lemon4ksan/g-man/pkg/steam/socket"
	"github.com/lemon4ksan/g-man/pkg/steam/socket/session"
	"github.com/lemon4ksan/g-man/pkg/storage/memory"
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

func (m *mockSocket) SetEncryptionKey(key []byte) bool {
	args := m.Called(key)
	return args.Bool(0)
}

func (m *mockSocket) StartHeartbeat(t time.Duration) error {
	args := m.Called(t)
	return args.Error(0)
}

func (m *mockSocket) Send(ctx context.Context, build socket.PayloadBuilder, opts ...socket.SendOption) error {
	args := m.Called(ctx, build, opts)
	return args.Error(0)
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

func (m *mockSocket) SendRaw(ctx context.Context, eMsg enums.EMsg, payload []byte, opts ...socket.SendOption) error {
	args := m.Called(ctx, eMsg, payload, opts)
	return args.Error(0)
}

func (m *mockSocket) SendSync(
	ctx context.Context,
	build socket.PayloadBuilder,
	opts ...socket.SendOption,
) (*protocol.Packet, error) {
	args := m.Called(ctx, build, opts)
	pkt, _ := args.Get(0).(*protocol.Packet)
	return pkt, args.Error(1)
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

func (m *mockSocket) Connect(ctx context.Context, server socket.CMServer) error {
	args := m.Called(ctx, server)
	return args.Error(0)
}

func (m *mockSocket) Session() socket.Session {
	args := m.Called()
	sess, _ := args.Get(0).(socket.Session)
	return sess
}

type mockSession struct {
	*session.Session
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

func (m *mockSession) IsAuthenticated() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *mockSession) SessionID() int32 {
	args := m.Called()
	return args.Get(0).(int32)
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

type testClient struct {
	session *Session
}

type testMocks struct {
	auth *mockAuthenticator
	web  *mockWebSession
	comm *mockCommunity
	sock *mockSocket
	http *mockHTTPDoer
}

func setupTestClient(_ *testing.T) (*testClient, *testMocks) {
	m := &testMocks{
		auth: new(mockAuthenticator),
		web:  new(mockWebSession),
		comm: new(mockCommunity),
		sock: new(mockSocket),
		http: new(mockHTTPDoer),
	}

	cfg := Config{
		Logger:        log.Discard,
		HTTP:          m.http,
		Authenticator: m.auth,
		WebFactory: func(steamID id.ID, logger log.Logger, baseDoer aoni.HTTPDoer) WebSessionProvider {
			return m.web
		},
		CommunityFactory: func(httpClient *http.Client, sessionID func(string) string, logger log.Logger) CommunityProvider {
			return m.comm
		},
	}

	sess := New(m.sock, cfg)
	sess.web = m.web
	sess.community = m.comm

	m.web.On("HTTP").Return(&http.Client{}).Maybe()

	return &testClient{session: sess}, m
}

func TestSession_ResolveDefaults(t *testing.T) {
	cfg := Config{}
	cfg.ResolveDefaults()

	assert.NotNil(t, cfg.Logger)
	assert.NotNil(t, cfg.Bus)
	assert.NotNil(t, cfg.Storage)
	assert.Equal(t, service.WebAPIBase, cfg.WebAPIBase)
	assert.NotNil(t, cfg.HTTP)
	assert.NotNil(t, cfg.Device)
	assert.NotNil(t, cfg.WebFactory)
	assert.NotNil(t, cfg.CommunityFactory)
}

func TestSession_ConstructorDefaultAuthenticator(t *testing.T) {
	msock := new(mockSocket)
	msock.On("RegisterMsgHandler", enums.EMsg_ChannelEncryptRequest, mock.Anything).Return()
	msock.On("RegisterMsgHandler", enums.EMsg_ChannelEncryptResult, mock.Anything).Return()
	msock.On("RegisterMsgHandler", enums.EMsg_ClientLogOnResponse, mock.Anything).Return()
	msock.On("RegisterMsgHandler", enums.EMsg_ClientLoggedOff, mock.Anything).Return()

	cfg := Config{
		Storage: memory.New(),
	}
	s := New(msock, cfg)
	assert.NotNil(t, s.auth)
	msock.AssertExpectations(t)
}

func TestSessionManager_LogOn(t *testing.T) {
	c, m := setupTestClient(t)

	server := socket.CMServer{Endpoint: "cm1.steam.com", Type: "tcp"}
	details := &auth.LogOnDetails{SteamID: 12345}

	m.auth.On("LogOn", mock.Anything, details, server).Return(nil).Once()
	m.web.On("Verify", mock.Anything).Return(true, nil)
	m.comm.On("GetOrRegisterAPIKey", mock.Anything, "g-man-bot.dev").Return("key_123", nil)

	err := c.session.LogOn(t.Context(), server, details)
	assert.NoError(t, err)

	assert.Eventually(t, func() bool {
		return c.session.unified.APIKey() == "key_123"
	}, time.Second, 10*time.Millisecond)
}

func TestSessionManager_Refresh(t *testing.T) {
	c, m := setupTestClient(t)

	m.web.On("Verify", mock.Anything).Return(false, nil).Once()

	msess := new(mockSession)
	msess.On("RefreshToken").Return("my_refresh_token")
	msess.On("SteamID").Return(uint64(12345))
	msess.On("SetAccessToken", "new_at").Return()
	msess.On("AccessToken").Return("new_at")
	msess.On("IsAuthenticated").Return(true)
	msess.On("SessionID").Return(int32(123))

	m.sock.On("Session").Return(msess)

	tokenPb, _ := proto.Marshal(&pb.CAuthentication_AccessToken_GenerateForApp_Response{
		AccessToken: proto.String("new_at"),
	})

	m.sock.On("SendSync", mock.Anything, mock.Anything, mock.Anything).Return(&protocol.Packet{
		IsProto: true,
		Header: &protocol.MsgHdrProtoBuf{
			Proto: &pb.CMsgProtoBufHeader{
				Eresult: proto.Int32(int32(enums.EResult_OK)),
			},
		},
		Payload: tokenPb,
	}, nil).Once()

	m.web.On("Authenticate",
		mock.Anything,
		mock.AnythingOfType("steam.EAuthTokenPlatformType"),
		"my_refresh_token",
		"new_at",
	).Return(nil).Once()

	err := c.session.Refresh(t.Context())

	assert.NoError(t, err)
	m.sock.AssertExpectations(t)
	m.web.AssertExpectations(t)
}

func TestSessionManager_LogOn_Errors(t *testing.T) {
	c, m := setupTestClient(t)
	server := socket.CMServer{}
	details := &auth.LogOnDetails{SteamID: 1}

	t.Run("Auth Fails", func(t *testing.T) {
		m.auth.On("LogOn", mock.Anything, details, server).Return(errors.New("auth nope")).Once()
		err := c.session.LogOn(t.Context(), server, details)
		assert.ErrorContains(t, err, "auth nope")
	})

	t.Run("Refresh Fails", func(t *testing.T) {
		m.auth.On("LogOn", mock.Anything, details, server).Return(nil).Once()
		c.session.closed.Store(true) // trigger module.ErrClosed in Refresh
		err := c.session.LogOn(t.Context(), server, details)
		assert.ErrorContains(t, err, module.ErrClosed.Error())
		c.session.closed.Store(false)
	})

	t.Run("API Key Fails", func(t *testing.T) {
		m.auth.On("LogOn", mock.Anything, details, server).Return(nil).Once()
		m.web.On("Verify", mock.Anything).Return(true, nil).Once()
		m.comm.On("GetOrRegisterAPIKey", mock.Anything, mock.Anything).Return("", errors.New("no api key")).Once()

		err := c.session.LogOn(t.Context(), server, details)
		assert.NoError(t, err)
	})

	t.Run("Nil details", func(t *testing.T) {
		err := c.session.LogOn(t.Context(), server, nil)
		assert.ErrorContains(t, err, "cannot login with nil credentials")
	})
}

func TestSessionManager_Refresh_Errors(t *testing.T) {
	t.Run("Already Closed", func(t *testing.T) {
		c, _ := setupTestClient(t)
		c.session.closed.Store(true)
		assert.ErrorIs(t, c.session.Refresh(t.Context()), module.ErrClosed)
		c.session.closed.Store(false)
	})

	t.Run("Web Session Valid", func(t *testing.T) {
		c, m := setupTestClient(t)
		m.web.On("Verify", t.Context()).Return(true, nil).Once()
		assert.NoError(t, c.session.Refresh(t.Context()))
	})

	t.Run("Generate Token Fails", func(t *testing.T) {
		c, m := setupTestClient(t)
		m.web.On("Verify", t.Context()).Return(false, nil).Once()

		msess := new(mockSession)
		msess.On("RefreshToken").Return("rt")
		msess.On("SteamID").Return(uint64(1))
		msess.On("IsAuthenticated").Return(true)
		msess.On("SessionID").Return(int32(123))
		m.sock.On("Session").Return(msess)

		m.sock.On("SendSync", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("socket err")).Once()

		err := c.session.Refresh(t.Context())
		assert.ErrorContains(t, err, "failed to generate access token")
	})

	t.Run("Web Auth Fails", func(t *testing.T) {
		c, m := setupTestClient(t)
		m.web.On("Verify", t.Context()).Return(false, nil).Once()

		msess := new(mockSession)
		msess.On("RefreshToken").Return("rt")
		msess.On("SteamID").Return(uint64(1))
		msess.On("AccessToken").Return("old")
		msess.On("SetAccessToken", "new_at").Return()
		msess.On("IsAuthenticated").Return(true)
		msess.On("SessionID").Return(int32(123))
		m.sock.On("Session").Return(msess)

		tokenPb, _ := proto.Marshal(
			&pb.CAuthentication_AccessToken_GenerateForApp_Response{AccessToken: proto.String("new_at")},
		)
		m.sock.On("SendSync", mock.Anything, mock.Anything, mock.Anything).Return(&protocol.Packet{
			IsProto: true,
			Header: &protocol.MsgHdrProtoBuf{
				Proto: &pb.CMsgProtoBufHeader{
					Eresult: proto.Int32(int32(enums.EResult_OK)),
				},
			},
			Payload: tokenPb,
		}, nil).Once()

		m.web.On("Authenticate", t.Context(), mock.Anything, "rt", mock.Anything).Return(errors.New("web nope")).Once()

		err := c.session.Refresh(t.Context())
		assert.ErrorContains(t, err, "web auth failed")
	})
}

func TestSessionManager_Refresh_MissingTokens(t *testing.T) {
	c, m := setupTestClient(t)

	m.web.On("Verify", t.Context()).Return(false, nil).Once()
	m.sock.On("Session").Return(nil)

	err := c.session.Refresh(t.Context())
	assert.ErrorIs(t, err, ErrMissingCredentials)
}

func TestSessionManager_Refresh_SocketDisconnectMidway(t *testing.T) {
	c, m := setupTestClient(t)

	m.web.On("Verify", t.Context()).Return(false, nil).Once()

	msess := new(mockSession)
	msess.On("RefreshToken").Return("rt")
	msess.On("SteamID").Return(uint64(1))
	msess.On("IsAuthenticated").Return(true).Maybe()
	msess.On("SessionID").Return(int32(123)).Maybe()

	m.sock.On("Session").Return(msess).Times(3)

	tokenPb, _ := proto.Marshal(&pb.CAuthentication_AccessToken_GenerateForApp_Response{
		AccessToken: proto.String("new_at"),
	})

	m.sock.On("SendSync", mock.Anything, mock.Anything, mock.Anything).Return(&protocol.Packet{
		IsProto: true,
		Header: &protocol.MsgHdrProtoBuf{
			Proto: &pb.CMsgProtoBufHeader{
				Eresult: proto.Int32(int32(enums.EResult_OK)),
			},
		},
		Payload: tokenPb,
	}, nil).Once()

	m.sock.On("Session").Return(nil).Once()

	err := c.session.Refresh(t.Context())
	assert.ErrorIs(t, err, ErrSocketNotConnected)
}

func TestSessionManager_LoopAndClose(t *testing.T) {
	c, m := setupTestClient(t)

	m.sock.On("Close").Return(nil).Once()

	err := c.session.Close()
	assert.NoError(t, err)
	assert.True(t, c.session.closed.Load())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	m.sock.On("Disconnect").Return(nil).Once()

	err = c.session.StartRefreshLoop(ctx)
	assert.NoError(t, err)
}

func TestSession_StartRefreshLoop_TriggerRefresh(t *testing.T) {
	c, m := setupTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())

	m.web.On("IsAuthenticated").Return(true)
	m.web.On("Verify", mock.Anything).Return(false, nil)

	msess := new(mockSession)
	msess.On("RefreshToken").Return("rt_loop")
	msess.On("SteamID").Return(uint64(1))
	msess.On("IsAuthenticated").Return(true).Maybe()
	msess.On("SessionID").Return(int32(123)).Maybe()
	msess.On("SetAccessToken", "at_loop").Return()
	m.sock.On("Session").Return(msess)

	tokenPb, _ := proto.Marshal(&pb.CAuthentication_AccessToken_GenerateForApp_Response{
		AccessToken: proto.String("at_loop"),
	})

	m.sock.On("SendSync", mock.Anything, mock.Anything, mock.Anything).Return(&protocol.Packet{
		IsProto: true,
		Header: &protocol.MsgHdrProtoBuf{
			Proto: &pb.CMsgProtoBufHeader{
				Eresult: proto.Int32(int32(enums.EResult_OK)),
			},
		},
		Payload: tokenPb,
	}, nil)

	m.web.On("Authenticate", mock.Anything, mock.Anything, "rt_loop", "at_loop").Return(nil)

	c.session.refreshJobInterval = 5 * time.Millisecond

	m.sock.On("Disconnect").Return(nil).Once()

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := c.session.StartRefreshLoop(ctx)
	assert.NoError(t, err)
}

func TestSession_StartRefreshLoop_TriggerRefreshFailure(t *testing.T) {
	c, m := setupTestClient(t)
	ctx, cancel := context.WithCancel(context.Background())

	m.web.On("IsAuthenticated").Return(true)
	m.web.On("Verify", mock.Anything).Return(false, nil)
	m.sock.On("Session").Return(nil)

	c.session.refreshJobInterval = 5 * time.Millisecond

	m.sock.On("Disconnect").Return(nil).Once()

	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	err := c.session.StartRefreshLoop(ctx)
	assert.NoError(t, err)
}

func TestSessionManager_CustomFactories(t *testing.T) {
	webCalled := false
	commCalled := false

	mw := new(mockWebSession)
	mw.On("Verify", mock.Anything).Return(true, nil).Maybe()
	mw.On("HTTP").Return(&http.Client{}).Maybe()

	mc := new(mockCommunity)
	mc.On("GetOrRegisterAPIKey", mock.Anything, mock.Anything).Return("key_12345", nil).Maybe()

	cfg := Config{
		WebFactory: func(steamID id.ID, logger log.Logger, baseDoer aoni.HTTPDoer) WebSessionProvider {
			webCalled = true
			return mw
		},
		CommunityFactory: func(httpClient *http.Client, sessionID func(string) string, logger log.Logger) CommunityProvider {
			commCalled = true
			return mc
		},
	}

	server := socket.CMServer{Endpoint: "cm1.steam.com", Type: "tcp"}
	details := &auth.LogOnDetails{SteamID: 12345}

	ma := new(mockAuthenticator)
	ma.On("LogOn", mock.Anything, details, server).Return(nil).Once()

	msock := new(mockSocket)
	sessionCfg := Config{
		Logger:           log.Discard,
		Authenticator:    ma, // <-- Pass the mock authenticator directly to the config!
		WebFactory:       cfg.WebFactory,
		CommunityFactory: cfg.CommunityFactory,
		Storage:          memory.New(),
	}

	sm := New(msock, sessionCfg)

	_ = sm.LogOn(t.Context(), server, details)

	assert.True(t, webCalled, "custom WebFactory should be invoked")
	assert.True(t, commCalled, "custom CommunityFactory should be invoked")
}

func TestSessionManager_Refresh_SingleFlight(t *testing.T) {
	c, m := setupTestClient(t)

	m.web.On("Verify", mock.Anything).Return(false, nil).Once()
	m.web.On("Verify", mock.Anything).Return(true, nil)

	msess := new(mockSession)
	msess.On("RefreshToken").Return("refresh_token_sf")
	msess.On("SteamID").Return(uint64(12345))
	msess.On("IsAuthenticated").Return(true).Maybe()
	msess.On("SessionID").Return(int32(123)).Maybe()
	msess.On("SetAccessToken", "new_token_sf").Return()
	msess.On("AccessToken").Return("new_token_sf")
	m.sock.On("Session").Return(msess)

	tokenPb, _ := proto.Marshal(&pb.CAuthentication_AccessToken_GenerateForApp_Response{
		AccessToken: proto.String("new_token_sf"),
	})

	m.sock.On("SendSync", mock.Anything, mock.Anything, mock.Anything).Return(&protocol.Packet{
		IsProto: true,
		Header: &protocol.MsgHdrProtoBuf{
			Proto: &pb.CMsgProtoBufHeader{
				Eresult: proto.Int32(int32(enums.EResult_OK)),
			},
		},
		Payload: tokenPb,
	}, nil).Once()

	m.web.On("Authenticate", mock.Anything, mock.Anything, "refresh_token_sf", "new_token_sf").Return(nil).Once()

	var wg sync.WaitGroup

	concurrentCount := 10
	wg.Add(concurrentCount)

	for range concurrentCount {
		go func() {
			defer wg.Done()

			err := c.session.Refresh(t.Context())
			assert.NoError(t, err)
		}()
	}

	wg.Wait()

	m.http.AssertExpectations(t)
	m.web.AssertExpectations(t)
}

func TestSession_GettersAndMutators(t *testing.T) {
	c, m := setupTestClient(t)

	assert.NotNil(t, c.session.Storage())

	m.sock.On("Session").Return(nil).Times(3)
	assert.Equal(t, id.ID(0), c.session.SteamID())
	assert.Equal(t, "", c.session.AccessToken())
	assert.Equal(t, "", c.session.RefreshToken())

	msess := new(mockSession)
	msess.On("SteamID").Return(uint64(123456))
	msess.On("AccessToken").Return("at_token")
	msess.On("RefreshToken").Return("rt_token")
	m.sock.On("Session").Return(msess).Times(3)

	assert.Equal(t, id.ID(123456), c.session.SteamID())
	assert.Equal(t, "at_token", c.session.AccessToken())
	assert.Equal(t, "rt_token", c.session.RefreshToken())

	c.session.community = nil

	m.web.On("HTTP").Return(&http.Client{})
	m.web.On("SessionID", mock.Anything).Return("sid_123")

	comm1 := c.session.Community()
	assert.NotNil(t, comm1)

	comm2 := c.session.Community()
	assert.Equal(t, comm1, comm2)

	c.session.web = nil

	m.sock.On("Session").Return(msess).Once()

	web1 := c.session.Web()
	assert.NotNil(t, web1)

	web2 := c.session.Web()
	assert.Equal(t, web1, web2)

	assert.NotNil(t, c.session.Socket())
	assert.NotNil(t, c.session.Unified())

	c.session.web = nil
	assert.False(t, c.session.IsAuthenticated())

	c.session.web = m.web
	m.web.On("IsAuthenticated").Return(true).Once()
	assert.True(t, c.session.IsAuthenticated())

	m.web.On("IsAuthenticated").Return(false).Once()
	assert.False(t, c.session.IsAuthenticated())

	m.sock.On("IsConnected").Return(true).Once()
	assert.True(t, c.session.IsSocketConnected())

	server := socket.CMServer{Endpoint: "cm.test"}
	c.session.SetLogonServer(server)
	assert.Equal(t, server, c.session.logonServer)

	c.session.SetAPIKey("test_key")
	assert.Equal(t, "test_key", c.session.Unified().APIKey())
	assert.Equal(t, "test_key", c.session.Socket().APIKey())

	m.sock.On("Session").Return(nil).Once()

	err := c.session.SetAccessToken("token")
	assert.ErrorIs(t, err, ErrSocketNotConnected)

	m.sock.On("Session").Return(msess).Once()
	msess.On("SetAccessToken", "new_at").Return().Once()

	err = c.session.SetAccessToken("new_at")
	assert.NoError(t, err)
	assert.Equal(t, "new_at", c.session.Unified().AccessToken())
	assert.Equal(t, "new_at", c.session.Socket().AccessToken())

	assert.NotNil(t, c.session.Logger())
}

func TestSession_Reconnect(t *testing.T) {
	c, m := setupTestClient(t)

	err := c.session.Reconnect(t.Context())
	assert.ErrorIs(t, err, ErrMissingCredentials)

	server := socket.CMServer{Endpoint: "cm.test"}
	details := &auth.LogOnDetails{SteamID: 12345}

	c.session.logonDetails = details
	c.session.logonServer = server

	m.auth.On("LogOn", t.Context(), details, server).Return(nil).Once()
	m.web.On("Verify", t.Context()).Return(true, nil).Once()
	m.comm.On("GetOrRegisterAPIKey", t.Context(), "g-man-bot.dev").Return("key_123", nil).Once()

	err = c.session.Reconnect(t.Context())
	assert.NoError(t, err)

	assert.Equal(t, m.web, c.session.web)
	assert.Equal(t, m.comm, c.session.community)
}

func TestSession_Reconnect_LogOnFailure(t *testing.T) {
	c, m := setupTestClient(t)

	server := socket.CMServer{Endpoint: "cm.test"}
	details := &auth.LogOnDetails{SteamID: 123}

	c.session.logonDetails = details
	c.session.logonServer = server

	m.auth.On("LogOn", t.Context(), details, server).Return(errors.New("login failed")).Once()

	err := c.session.Reconnect(t.Context())
	assert.ErrorContains(t, err, "session: login failed: login failed")
}

func TestSession_Verify(t *testing.T) {
	c, m := setupTestClient(t)

	m.web.On("Verify", t.Context()).Return(true, nil).Once()
	ok, err := c.session.Verify(t.Context())
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestSession_EnrichLogger(t *testing.T) {
	c, _ := setupTestClient(t)

	assert.Equal(t, "", c.session.enrichedAccount)
	assert.Equal(t, id.ID(0), c.session.enrichedSteamID)

	c.session.EnrichLogger("user123", id.ID(999))
	assert.Equal(t, "user123", c.session.enrichedAccount)
	assert.Equal(t, id.ID(999), c.session.enrichedSteamID)

	c.session.EnrichLogger("user456", id.ID(111))
	assert.Equal(t, "user123", c.session.enrichedAccount)
	assert.Equal(t, id.ID(999), c.session.enrichedSteamID)
}

func TestSession_Disconnect(t *testing.T) {
	c, m := setupTestClient(t)
	m.sock.On("Disconnect").Return(errors.New("disconnect err")).Once()

	err := c.session.Disconnect()
	assert.ErrorContains(t, err, "disconnect err")
}
