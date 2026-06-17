// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package httpc

import (
	"net/http"
	"time"

	"github.com/lemon4ksan/aoni"

	"github.com/lemon4ksan/g-man/pkg/log"
)

// LoggingMiddleware returns a middleware that logs HTTP requests using the provided logger.
func LoggingMiddleware(logger log.Logger) aoni.Middleware {
	return func(next aoni.HTTPDoer) aoni.HTTPDoer {
		return aoni.DoerFunc(func(req *http.Request) (*http.Response, error) {
			start := time.Now()
			resp, err := next.Do(req)

			logger.Info("http request",
				log.String("method", req.Method),
				log.String("url", req.URL.String()),
				log.Duration("duration", time.Since(start)),
				log.Err(err),
			)

			return resp, err
		})
	}
}
