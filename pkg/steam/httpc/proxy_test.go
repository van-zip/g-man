// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package httpc

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSteamStickyKey(t *testing.T) {
	t.Run("From cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com", nil)
		req.AddCookie(&http.Cookie{Name: "sessionid", Value: "abc123"})

		key := SteamStickyKey(req)
		assert.Equal(t, "abc123", key)
	})

	t.Run("From header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com", nil)
		req.Header.Set("X-Steam-SessionID", "header123")

		key := SteamStickyKey(req)
		assert.Equal(t, "header123", key)
	})

	t.Run("Cookie takes precedence over header", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com", nil)
		req.AddCookie(&http.Cookie{Name: "sessionid", Value: "cookie123"})
		req.Header.Set("X-Steam-SessionID", "header123")

		key := SteamStickyKey(req)
		assert.Equal(t, "cookie123", key)
	})

	t.Run("Returns empty when not present", func(t *testing.T) {
		req := httptest.NewRequest("GET", "http://example.com", nil)

		key := SteamStickyKey(req)
		assert.Empty(t, key)
	})
}
