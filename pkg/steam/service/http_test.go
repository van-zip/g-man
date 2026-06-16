// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package service

import (
	"net/url"
	"testing"

	"github.com/lemon4ksan/g-man/pkg/steam/encoding"
	tr "github.com/lemon4ksan/g-man/pkg/steam/transport"
)

type mockHTTPTarget struct {
	URL        string
	HTTPMethod string
	Version    int
}

func (m *mockHTTPTarget) String() string         { return m.URL }
func (m *mockHTTPTarget) SetHTTPMethod(s string) { m.HTTPMethod = s }
func (m *mockHTTPTarget) SetVersion(v int)       { m.Version = v }

func TestCallOptions_Detailed(t *testing.T) {
	target := &mockHTTPTarget{URL: "http://api.steampowered.com"}
	req := tr.NewRequest(target, nil)
	cfg := &CallConfig{}

	t.Run("WithHeader", func(t *testing.T) {
		WithHeader("X-Custom-Header", "G-Man-Secret")(req, cfg)

		if req.Header().Get("X-Custom-Header") != "G-Man-Secret" {
			t.Error("WithHeader failed to set header")
		}
	})

	t.Run("WithCustomRegistry", func(t *testing.T) {
		customReg := &encoding.UnmarshalRegistry{}
		WithCustomRegistry(customReg)(req, cfg)

		if cfg.Registry != customReg {
			t.Error("WithCustomRegistry failed to set registry")
		}
	})

	t.Run("WithHTTPMethod - Compatibility check", func(t *testing.T) {
		reqNoSetter := tr.NewRequest(nil, nil)
		WithHTTPMethod("POST")(reqNoSetter, cfg)
	})

	t.Run("WithVersion - Compatibility check", func(t *testing.T) {
		reqNoSetter := tr.NewRequest(nil, nil)
		WithVersion(2)(reqNoSetter, cfg)
	})
}

func TestHttpTarget(t *testing.T) {
	target := HTTPTarget{
		Method: "POST",
		URL:    "https://steamcommunity.com/tradeoffer/new/",
	}

	if target.HTTPMethod() != "POST" {
		t.Errorf("expected POST, got %s", target.HTTPMethod())
	}

	expectedPath := "tradeoffer/new/"
	if target.HTTPPath() != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, target.HTTPPath())
	}

	// Test default method
	targetDefault := HTTPTarget{URL: "http://test.com/path"}
	if targetDefault.HTTPMethod() != "GET" {
		t.Error("expected default method GET")
	}
}

func TestHttpTarget_Methods(t *testing.T) {
	t.Run("Default Method", func(t *testing.T) {
		target := HTTPTarget{URL: "http://test.com"}
		if target.HTTPMethod() != "GET" {
			t.Errorf("expected default GET, got %s", target.HTTPMethod())
		}
	})

	t.Run("Custom Method", func(t *testing.T) {
		target := HTTPTarget{URL: "http://test.com", Method: "PATCH"}
		if target.HTTPMethod() != "PATCH" {
			t.Errorf("expected PATCH, got %s", target.HTTPMethod())
		}
	})

	t.Run("String representation", func(t *testing.T) {
		u := "http://example.com/api"

		target := HTTPTarget{URL: u}
		if target.String() != u {
			t.Errorf("expected %s, got %s", u, target.String())
		}
	})

	t.Run("HTTPPath extraction", func(t *testing.T) {
		tests := []struct {
			url      string
			expected string
		}{
			{"http://api.com/ISteamUser/GetPlayerSummaries/v0002/", "ISteamUser/GetPlayerSummaries/v0002/"},
			{"/v1/internal/call", "v1/internal/call"},
			{"api/no-slash", "api/no-slash"},
			{"", ""},
		}

		for _, tc := range tests {
			target := HTTPTarget{URL: tc.url}
			if target.HTTPPath() != tc.expected {
				t.Errorf("url: %s, expected path: %s, got: %s", tc.url, tc.expected, target.HTTPPath())
			}
		}
	})
}

func TestNewHttpRequest_Creation(t *testing.T) {
	method := "DELETE"
	url := "http://steam.com/delete"
	body := []byte("payload")

	req := NewHTTPRequest(method, url, body)

	if req == nil {
		t.Fatal("request is nil")
	}

	target, ok := req.Target().(HTTPTarget)
	if !ok {
		t.Fatal("target should be HttpTarget")
	}

	if target.Method != method {
		t.Errorf("expected %s, got %s", method, target.Method)
	}

	if string(req.Body()) != "payload" {
		t.Errorf("expected payload, got %s", string(req.Body()))
	}
}

func TestNewHttpRequest(t *testing.T) {
	req := NewHTTPRequest("POST", "http://example.com/api", []byte("body"))

	target, ok := req.Target().(HTTPTarget)
	if !ok {
		t.Fatal("target is not HttpTarget")
	}

	if target.HTTPMethod() != "POST" || target.HTTPPath() != "api" {
		t.Errorf("NewHttpRequest created invalid target: %+v", target)
	}
}

func TestOptions_NonCompatibleTarget(t *testing.T) {
	req := NewHTTPRequest("GET", "http://a.b", nil)

	WithVersion(2)(req, &CallConfig{})
	WithHTTPMethod("POST")(req, &CallConfig{})
}

func TestCallOptions(t *testing.T) {
	target := &mockHTTPTarget{URL: "test"}
	req := tr.NewRequest(target, nil)
	cfg := &CallConfig{}

	t.Run("WithHTTPMethod", func(t *testing.T) {
		WithHTTPMethod("PUT")(req, cfg)

		if target.HTTPMethod != "PUT" {
			t.Error("WithHTTPMethod failed")
		}
	})

	t.Run("WithVersion", func(t *testing.T) {
		WithVersion(5)(req, cfg)

		if target.Version != 5 {
			t.Error("WithVersion failed")
		}
	})

	t.Run("WithFormat", func(t *testing.T) {
		WithFormat(encoding.FormatVDF)(req, cfg)

		if cfg.Format != encoding.FormatVDF {
			t.Error("WithFormat failed")
		}
	})

	t.Run("QueryParams", func(t *testing.T) {
		WithQueryParam("a", "1")(req, cfg)
		WithQueryParams(url.Values{"b": {"2"}})(req, cfg)
		WithOverrideAPIKey("secret")(req, cfg)

		params := req.Params()
		if params.Get("a") != "1" || params.Get("b") != "2" || params.Get("key") != "secret" {
			t.Errorf("query params injection failed: %v", params)
		}
	})
}

func TestResponseFormat_Values(t *testing.T) {
	formats := []encoding.ResponseFormat{
		encoding.FormatUnknown, encoding.FormatRaw, encoding.FormatProtobuf,
		encoding.FormatJSON, encoding.FormatVDF, encoding.FormatBinaryKV,
	}

	for i, f := range formats {
		if int(f) != i {
			t.Errorf("Format enum mismatch at index %d", i)
		}
	}
}
