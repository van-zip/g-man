// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package log

import (
	"net/http"
	"net/url"
	"time"

	"github.com/lemon4ksan/aoni"
)

// LoggingMiddleware returns a middleware that logs HTTP requests using the provided logger.
func LoggingMiddleware(logger Logger) aoni.Middleware {
	return func(next aoni.HTTPDoer) aoni.HTTPDoer {
		return aoni.DoerFunc(func(req *http.Request) (*http.Response, error) {
			start := time.Now()
			resp, err := next.Do(req)

			logger.Info("http request",
				String("method", req.Method),
				String("url", maskQueryParams(req.URL)),
				Duration("duration", time.Since(start)),
				Err(err),
			)

			return resp, err
		})
	}
}

// maskQueryParams returns the URL with sensitive query parameters masked.
func maskQueryParams(u *url.URL) string {
	if u == nil {
		return ""
	}

	copy := *u

	q := copy.Query()
	for key := range q {
		if key == "key" || key == "access_token" || key == "token" {
			q.Set(key, "***")
		}
	}

	copy.RawQuery = q.Encode()

	return copy.String()
}
