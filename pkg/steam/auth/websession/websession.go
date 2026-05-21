// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package websession provides a high-level interface for managing Steam web sessions.
//
// It automates the process of obtaining and synchronizing authentication cookies
// ('steamLoginSecure' and 'sessionid') across multiple Steam domains such as
// steamcommunity.com and steampowered.com.
package websession

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
	"time"

	"github.com/lemon4ksan/g-man/pkg/log"
	pb "github.com/lemon4ksan/g-man/pkg/protobuf/steam"
	"github.com/lemon4ksan/g-man/pkg/rest"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol/enums"
)

var defaultDomains = []string{
	"https://steamcommunity.com",
	"https://store.steampowered.com",
	"https://help.steampowered.com",
	"https://login.steampowered.com",
	"https://s.team",
}

const (
	urlFinalize            = "https://login.steampowered.com/jwt/finalizelogin"
	urlVerify              = "https://steamcommunity.com/chat/clientinterfaces"
	cookieSessionID        = "sessionid"
	cookieSteamLoginSecure = "steamLoginSecure"
)

// WebSession handles HTTP-based interactions with Steam Community and Store.
// It manages a shared cookie jar and provides a thread-safe way to authenticate.
//
// It implements the [rest.HTTPDoer] interface, allowing it to be used
// as a transport for REST clients that require session-aware cookies.
type WebSession struct {
	mu sync.RWMutex

	steamID    id.ID
	baseDoer   rest.HTTPDoer
	httpClient *http.Client
	jar        http.CookieJar
	logger     log.Logger
	isAuth     bool
	domains    []string
}

type doerRoundTripper struct {
	doer rest.HTTPDoer
}

func (d *doerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return d.doer.Do(req)
}

// New creates a new, unauthenticated web session for the provided SteamID.
func New(steamID id.ID, logger log.Logger, baseDoer rest.HTTPDoer) *WebSession {
	ws := &WebSession{
		steamID:  steamID,
		baseDoer: baseDoer,
		logger:   logger.With(log.Module("websession")),
		domains:  append([]string{}, defaultDomains...),
	}

	if baseDoer == nil {
		ws.baseDoer = &http.Client{Timeout: 30 * time.Second}
	}

	ws.httpClient = &http.Client{
		Transport: &doerRoundTripper{doer: ws.baseDoer},
		Timeout:   30 * time.Second,
	}

	ws.Clear()

	return ws
}

// Do implements [rest.HTTPDoer]. It executes the request using the session's
// internal cookie-aware HTTP client.
func (s *WebSession) Do(req *http.Request) (*http.Response, error) {
	s.mu.RLock()
	client := s.httpClient
	s.mu.RUnlock()

	// #nosec G704
	return client.Do(req)
}

// REST returns a new REST client instance configured to use this session.
func (s *WebSession) REST() *rest.Client {
	return rest.NewClient(s)
}

// HTTP returns the raw cookie-aware http.Client.
func (s *WebSession) HTTP() *http.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.httpClient
}

// AddDomains appends additional URLs to the session's synchronization list.
func (s *WebSession) AddDomains(domains ...string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.domains = append(s.domains, domains...)
}

// Authenticate synchronizes the web session with Steam's OIDC providers.
func (s *WebSession) Authenticate(
	ctx context.Context,
	platform pb.EAuthTokenPlatformType,
	refreshToken, accessToken string,
) error {
	if refreshToken == "" {
		return errors.New("websession: refresh token is required")
	}

	s.Clear()

	sessionID := generateSessionID()

	if platform == pb.EAuthTokenPlatformType_k_EAuthTokenPlatformType_SteamClient ||
		platform == pb.EAuthTokenPlatformType_k_EAuthTokenPlatformType_MobileApp && accessToken != "" {
		return s.applyFastPath(accessToken, sessionID)
	}

	return s.authSlowPath(ctx, refreshToken, sessionID)
}

// Verify proactively checks if the current web session is still valid.
func (s *WebSession) Verify(ctx context.Context) (bool, error) {
	if !s.IsAuthenticated() {
		return false, nil
	}

	resp, err := s.REST().Request(ctx, http.MethodGet, urlVerify, nil, nil)
	if err != nil {
		return false, fmt.Errorf("verify request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK || resp.Request.URL.Path == "/login/home/" {
		s.Clear()
		return false, nil
	}

	return true, nil
}

// IsAuthenticated returns true if the session has successfully obtained login cookies.
func (s *WebSession) IsAuthenticated() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.isAuth
}

// SessionID retrieves the value of the 'sessionid' cookie for a specific target URL.
func (s *WebSession) SessionID(targetURL string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, err := url.Parse(targetURL)
	if err != nil {
		return ""
	}

	for _, cookie := range s.jar.Cookies(u) {
		if cookie.Name == cookieSessionID {
			return cookie.Value
		}
	}

	return ""
}

// Clear completely resets the web session state.
func (s *WebSession) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	jar, _ := cookiejar.New(nil)
	s.jar = jar
	s.httpClient.Jar = jar
	s.isAuth = false
}

func (s *WebSession) applyFastPath(accessToken, sessionID string) error {
	secureCookieValue := fmt.Sprintf("%d||%s", s.steamID, accessToken)
	s.seedCookies(sessionID, secureCookieValue)

	s.mu.Lock()
	s.isAuth = true
	s.mu.Unlock()

	return nil
}

func (s *WebSession) authSlowPath(ctx context.Context, refreshToken, sessionID string) error {
	params := map[string]string{
		"nonce":         refreshToken,
		cookieSessionID: sessionID,
		"redir":         "https://steamcommunity.com/login/home/?goto=",
	}

	type finalizeResponse struct {
		Error        int `json:"error"`
		TransferInfo []struct {
			URL    string            `json:"url"`
			Params map[string]string `json:"params"`
		} `json:"transfer_info"`
	}

	res, err := rest.PostJSON[map[string]string, finalizeResponse](ctx, s.REST(), urlFinalize, params, nil)
	if err != nil {
		return fmt.Errorf("websession: finalize login failed: %w", err)
	}

	if res.Error != 0 {
		return fmt.Errorf("websession: finalize login error code: %d", res.Error)
	}

	for _, transfer := range res.TransferInfo {
		transferParams := map[string]string{"steamID": fmt.Sprintf("%d", s.steamID)}
		maps.Copy(transferParams, transfer.Params)

		if err := s.executeTransferWithRetry(ctx, transfer.URL, transferParams); err != nil {
			return err
		}
	}

	s.seedCookies(sessionID, "")

	s.mu.Lock()
	s.isAuth = true
	s.mu.Unlock()

	return nil
}

func (s *WebSession) executeTransferWithRetry(
	ctx context.Context,
	transferURL string,
	params map[string]string,
) error {
	const maxRetries = 3

	type transferResult struct {
		Result enums.EResult `json:"result"`
	}

	var lastErr error
	for range maxRetries {
		res, err := rest.PostJSON[map[string]string, transferResult](ctx, s.REST(), transferURL, params, nil)
		if err != nil {
			lastErr = err
			continue
		}

		if res.Result != enums.EResult_OK {
			return fmt.Errorf("steam error: %s", res.Result.String())
		}

		return nil
	}

	return fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
}

func (s *WebSession) seedCookies(sessionID, secureValue string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, domain := range s.domains {
		u, _ := url.Parse(domain)

		cookies := []*http.Cookie{
			{
				Name:     cookieSessionID,
				Value:    sessionID,
				Path:     "/",
				Secure:   true,
				HttpOnly: false,
				SameSite: http.SameSiteLaxMode,
			},
		}
		if secureValue != "" {
			cookies = append(cookies, &http.Cookie{
				Name:     cookieSteamLoginSecure,
				Value:    secureValue,
				Path:     "/",
				Secure:   true,
				HttpOnly: true,
				SameSite: http.SameSiteNoneMode,
			})
		}

		s.jar.SetCookies(u, cookies)
	}
}

func generateSessionID() string {
	var b [12]byte

	_, _ = rand.Read(b[:])

	return hex.EncodeToString(b[:])
}
