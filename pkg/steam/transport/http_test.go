// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package transport

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man/pkg/steam/protocol/enums"
)

type mockHTTPDoer struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPDoer) Do(req *http.Request) (*http.Response, error) {
	return m.doFunc(req)
}

type mockHTTPTarget struct {
	method string
	path   string
}

func (m mockHTTPTarget) String() string     { return "mock" }
func (m mockHTTPTarget) HTTPPath() string   { return m.path }
func (m mockHTTPTarget) HTTPMethod() string { return m.method }

type faultyReader struct{}

func (f faultyReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read failure")
}

func TestNewHTTPTransport(t *testing.T) {
	doer := &mockHTTPDoer{}
	tr := NewHTTPTransport(doer, "https://api.example.com")

	assert.NotNil(t, tr.client)
}

func TestHTTPTransport_ParseEResult(t *testing.T) {
	tr := &HTTPTransport{}

	tests := []struct {
		name     string
		header   string
		expected enums.EResult
	}{
		{"Valid Result OK", "1", enums.EResult_OK},
		{"Valid Result Fail", "2", enums.EResult_Fail},
		{"Missing Header", "", enums.EResult_OK},
		{"Invalid Integer", "abc", enums.EResult_OK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{Header: make(http.Header)}
			if tt.header != "" {
				resp.Header.Set("x-eresult", tt.header)
			}

			assert.Equal(t, tt.expected, tr.parseEResult(resp))
		})
	}
}

type nonHTTPTarget struct{}

func (n nonHTTPTarget) String() string { return "non-http" }

func TestHTTPTransport_Do(t *testing.T) {
	ctx := context.Background()
	baseURL := "https://api.steampowered.com"

	t.Run("Successful Request with Body and Headers", func(t *testing.T) {
		payload := []byte("hello steam")
		encodedPayload := base64.StdEncoding.EncodeToString(payload)

		doer := &mockHTTPDoer{
			doFunc: func(req *http.Request) (*http.Response, error) {
				assert.Equal(t, "POST", req.Method)
				assert.Contains(t, req.URL.String(), "input_protobuf_encoded="+url.QueryEscape(encodedPayload))
				assert.Equal(t, "Valve/Steam HTTP Client 1.0", req.Header.Get("User-Agent"))
				assert.Equal(t, "text/html,*/*;q=0.9", req.Header.Get("Accept"))
				assert.Equal(t, "val1", req.Header.Get("X-Custom-Header"))
				assert.Equal(t, []string{"multi1", "multi2"}, req.Header.Values("X-Multi"))

				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"x-eresult": {"1"}},
					Body:       io.NopCloser(bytes.NewReader([]byte("response_body"))),
				}, nil
			},
		}

		tr := NewHTTPTransport(doer, baseURL)
		req := NewRequest(mockHTTPTarget{method: "POST", path: "/test"}, bytes.NewReader(payload))
		req.WithHeader("X-Custom-Header", "val1")
		req.WithHeader("X-Multi", "multi1")
		req.WithHeader("X-Multi", "multi2")

		resp, err := tr.Do(ctx, req)
		require.NoError(t, err)

		bodyBytes, _ := io.ReadAll(resp.Body)
		assert.Equal(t, "response_body", string(bodyBytes))

		meta, ok := resp.HTTP()
		require.True(t, ok)
		assert.Equal(t, enums.EResult_OK, meta.Result)
		assert.Equal(t, http.StatusOK, meta.StatusCode)
	})

	t.Run("Empty Body", func(t *testing.T) {
		doer := &mockHTTPDoer{
			doFunc: func(req *http.Request) (*http.Response, error) {
				assert.Empty(t, req.URL.Query().Get("input_protobuf_encoded"))

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte("{}"))),
				}, nil
			},
		}
		tr := NewHTTPTransport(doer, baseURL)
		req := NewRequest(mockHTTPTarget{method: "GET", path: "/test"}, nil)
		_, err := tr.Do(ctx, req)
		assert.NoError(t, err)
	})

	t.Run("Unsupported Target Type", func(t *testing.T) {
		tr := NewHTTPTransport(&mockHTTPDoer{}, baseURL)
		req := NewRequest(nonHTTPTarget{}, nil)

		resp, err := tr.Do(ctx, req)
		assert.Nil(t, resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "does not support HTTP transport")
	})

	t.Run("Rest Client Request Error", func(t *testing.T) {
		expectedErr := errors.New("network timeout")
		doer := &mockHTTPDoer{
			doFunc: func(req *http.Request) (*http.Response, error) {
				return nil, expectedErr
			},
		}
		tr := NewHTTPTransport(doer, baseURL)
		req := NewRequest(mockHTTPTarget{method: "GET", path: "/test"}, nil)

		resp, err := tr.Do(ctx, req)
		assert.Nil(t, resp)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("Body Read Error", func(t *testing.T) {
		doer := &mockHTTPDoer{
			doFunc: func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(faultyReader{}),
				}, nil
			},
		}
		tr := NewHTTPTransport(doer, baseURL)
		req := NewRequest(mockHTTPTarget{method: "GET", path: "/test"}, nil)

		resp, err := tr.Do(ctx, req)
		if err != nil {
			assert.Contains(t, err.Error(), "failed to read response")
		} else {
			_, readErr := io.ReadAll(resp.Body)
			assert.Error(t, readErr)
			assert.Contains(t, readErr.Error(), "read failure")
		}
	})
}
