// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rest

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

// ProxyConfig contains parameters for configuring an HTTP client with proxy support.
type ProxyConfig struct {
	ProxyURL           string        // Format: http://user:pass@ip:port or socks5://ip:port
	Timeout            time.Duration // Overall request timeout (recommended 15-30s for proxies)
	InsecureSkipVerify bool          // Disable SSL verification
}

// NewProxyClient creates a standard *http.Client configured to work through a proxy.
// It safely manages the connection pool to avoid memory leaks when running bots.
func NewProxyClient(cfg ProxyConfig) (*http.Client, error) {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}

	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		// #nosec G402 -- InsecureSkipVerify is configurable by the user for proxy compatibility.
		TLSClientConfig: &tls.Config{InsecureSkipVerify: cfg.InsecureSkipVerify},
	}

	if cfg.ProxyURL != "" {
		u, err := url.Parse(cfg.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("rest: invalid proxy URL %q: %w", cfg.ProxyURL, err)
		}

		// Go natively supports http://, https:// и socks5://
		transport.Proxy = http.ProxyURL(u)
	}

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}, nil
}

// ProxyRotatorConfig defines proxy health checking parameters.
type ProxyRotatorConfig struct {
	MaxFails   uint32        // How many errors in a row are allowed before shutdown (for example, 3)
	RetryAfter time.Duration // The time for which the proxy is excluded from the list (for example, 1 minute)
}

// StickyKeyFunc defines how to extract a session identifier from a request.
// If it returns an empty string, the request is handled via standard Round-Robin.
type StickyKeyFunc func(req *http.Request) string

// SteamStickyKey is a standard implementation for Steam-based requests.
// It looks for the 'sessionid' cookie or 'X-Steam-SessionID' header.
func SteamStickyKey(req *http.Request) string {
	if cookie, err := req.Cookie("sessionid"); err == nil {
		return cookie.Value
	}

	if sid := req.Header.Get("X-Steam-SessionID"); sid != "" {
		return sid
	}

	return ""
}

type trackedClient struct {
	client      HTTPDoer
	failCount   atomic.Uint32
	unhealthy   atomic.Bool
	recoveredAt atomic.Int64
}

// ProxyRotator allows distributing requests between multiple proxies.
// Implements the HTTPDoer interface, so it can be passed to [NewClient].
type ProxyRotator struct {
	clients []*trackedClient
	config  ProxyRotatorConfig
	current atomic.Uint64

	stickyKeyFunc StickyKeyFunc
	sessions      sync.Map // map[string]int (index in clients)
}

// NewProxyRotator initializes the rotator (Round-Robin).
func NewProxyRotator(config ProxyRotatorConfig, clients ...HTTPDoer) (*ProxyRotator, error) {
	if len(clients) == 0 {
		return nil, errors.New("rest: proxy rotator requires at least one client")
	}

	if config.MaxFails == 0 {
		config.MaxFails = 3
	}

	if config.RetryAfter == 0 {
		config.RetryAfter = 30 * time.Second
	}

	tracked := make([]*trackedClient, len(clients))
	for i, c := range clients {
		tracked[i] = &trackedClient{client: c}
	}

	return &ProxyRotator{
		clients: tracked,
		config:  config,
	}, nil
}

// WithStickySessions enables sticky sessions using the provided key extractor.
// Returns the copy of the proxy rotator with the sticky key function set.
func (r *ProxyRotator) WithStickySessions(f StickyKeyFunc) *ProxyRotator {
	c := &ProxyRotator{
		clients:       make([]*trackedClient, 0, len(r.clients)),
		config:        r.config,
		stickyKeyFunc: f,
	}
	copy(c.clients, r.clients)
	c.current.Store(r.current.Load())

	return c
}

// Do performs an HTTP request using the next available client in the rotation (Round-Robin).
// If sticky sessions are enabled, it attempts to use the same proxy for the same session ID.
func (r *ProxyRotator) Do(req *http.Request) (*http.Response, error) {
	var (
		lastErr   error
		n         = uint64(len(r.clients))
		sessionID string
		stickyIdx = -1
	)

	// Attempt to extract session ID and find a "stuck" proxy
	if r.stickyKeyFunc != nil {
		sessionID = r.stickyKeyFunc(req)
		if sessionID != "" {
			if val, ok := r.sessions.Load(sessionID); ok {
				stickyIdx = val.(int)
			}
		}
	}

	// Try the sticky proxy first if it's available
	if stickyIdx >= 0 && stickyIdx < len(r.clients) {
		tc := r.clients[stickyIdx]
		if r.isAvailable(tc) {
			resp, err := tc.client.Do(req)
			if !r.isProxyFault(resp, err) {
				r.markSuccess(tc)

				return resp, err
			}

			// Sticky proxy failed, mark it and move to general rotation
			r.markFailed(tc)

			if resp != nil {
				_ = resp.Body.Close()
			}

			lastErr = err
		}
	}

	// General Round-Robin rotation
	for range n {
		idx := r.current.Add(1) % n
		if int(idx) == stickyIdx {
			continue // Already tried above
		}

		tc := r.clients[idx]
		if !r.isAvailable(tc) {
			continue
		}

		resp, err := tc.client.Do(req)
		if r.isProxyFault(resp, err) {
			r.markFailed(tc)

			lastErr = err

			if resp != nil {
				_ = resp.Body.Close()
			}

			continue
		}

		r.markSuccess(tc)

		// Update or set the sticky association for future requests
		if sessionID != "" {
			r.sessions.Store(sessionID, int(idx))
		}

		return resp, err
	}

	if lastErr != nil {
		return nil, fmt.Errorf("rest: all proxies failed, last error: %w", lastErr)
	}

	return nil, errors.New("rest: no healthy proxies available")
}

func (r *ProxyRotator) isAvailable(tc *trackedClient) bool {
	if !tc.unhealthy.Load() {
		return true
	}

	if time.Now().UnixNano() >= tc.recoveredAt.Load() {
		return true
	}

	return false
}

func (r *ProxyRotator) markFailed(tc *trackedClient) {
	fails := tc.failCount.Add(1)
	if fails >= r.config.MaxFails {
		tc.unhealthy.Store(true)
		recoveryTime := time.Now().Add(r.config.RetryAfter).UnixNano()
		tc.recoveredAt.Store(recoveryTime)
	}
}

func (r *ProxyRotator) markSuccess(tc *trackedClient) {
	tc.failCount.Store(0)
	tc.unhealthy.Store(false)
}

func (r *ProxyRotator) isProxyFault(resp *http.Response, err error) bool {
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) {
			return true
		}

		return true
	}

	if resp != nil {
		if resp.StatusCode == http.StatusProxyAuthRequired { // 407
			return true
		}

		if resp.StatusCode == http.StatusTooManyRequests { // 429
			return true
		}

		if resp.StatusCode == http.StatusBadGateway ||
			resp.StatusCode == http.StatusGatewayTimeout ||
			resp.StatusCode == http.StatusServiceUnavailable {
			return true
		}
	}

	return false
}
