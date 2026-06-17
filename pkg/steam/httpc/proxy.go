// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package httpc

import "net/http"

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
