// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package community

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/lemon4ksan/aoni"

	"github.com/lemon4ksan/g-man/pkg/steam/community/client"
	"github.com/lemon4ksan/g-man/pkg/steam/encoding"
)

// BaseURL is the base url for community requests.
const BaseURL = client.BaseURL

// Requester defines the requirements for making Community requests.
type Requester = client.Requester

// SessionProvider defines how the community client retrieves active Steam session IDs.
type SessionProvider = client.SessionProvider

// NewClient creates a new Community Client.
var NewClient = client.New

var (
	// WithREST sets the REST client for the Community Client.
	WithREST = client.WithREST
	// WithLogger sets the logger for the Community Client.
	WithLogger = client.WithLogger
)

// Decorate wraps an existing Requester and adds global request modifiers.
func Decorate(r Requester, mods ...aoni.RequestModifier) Requester {
	if len(mods) == 0 {
		return r
	}

	return &decoratedRequester{
		Requester:   r,
		defaultMods: mods,
	}
}

// Get performs a GET request and unmarshals the resulting JSON into the Resp type.
func Get[Resp any](
	ctx context.Context,
	r Requester,
	path string,
	reqMsg any,
	mods ...aoni.RequestModifier,
) (*Resp, error) {
	var allMods []aoni.RequestModifier

	allMods = append(allMods, aoni.WithDecoder(encoding.SteamJSONDecoder))
	allMods = append(allMods, aoni.WithAccept("application/json, text/javascript; q=0.01"))
	allMods = append(allMods, aoni.WithHeader("X-Requested-With", "XMLHttpRequest"))

	if reqMsg != nil {
		allMods = append(allMods, aoni.WithQuery(reqMsg))
	}

	allMods = append(allMods, mods...)

	return aoni.GetJSON[Resp](ctx, r, path, allMods...)
}

// GetHTML performs a GET request specifically for raw HTML content.
func GetHTML(ctx context.Context, r Requester, path string, mods ...aoni.RequestModifier) (io.ReadCloser, error) {
	allMods := make([]aoni.RequestModifier, 0, 1+len(mods))
	allMods = append(
		allMods,
		aoni.WithAccept("text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"),
	)
	allMods = append(allMods, mods...)

	resp, err := r.Request(ctx, http.MethodGet, path, allMods...)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

// PostForm performs a POST request with application/x-www-form-urlencoded data.
// It automatically injects the "sessionid" into the form parameters.
func PostForm[Resp any](
	ctx context.Context,
	r Requester,
	path string,
	reqMsg any,
	mods ...aoni.RequestModifier,
) (*Resp, error) {
	var params url.Values

	if reqMsg != nil {
		var err error

		params, err = aoni.StructToValues(reqMsg)
		if err != nil {
			return nil, err
		}
	} else {
		params = make(url.Values)
	}

	if params.Get("sessionid") == "" {
		params.Set("sessionid", r.SessionID(BaseURL))
	}

	allMods := make([]aoni.RequestModifier, 0, 3+len(mods))
	allMods = append(allMods, aoni.WithContentType("application/x-www-form-urlencoded; charset=UTF-8"))
	allMods = append(allMods, aoni.WithAccept("application/json, text/javascript; q=0.01"))
	allMods = append(allMods, aoni.WithDecoder(encoding.SteamJSONDecoder))
	allMods = append(allMods, mods...)

	return aoni.PostForm[Resp](ctx, r, path, strings.NewReader(params.Encode()), allMods...)
}

// PostJSON performs a POST request with a JSON body.
// It automatically injects the "sessionid" into the URL query parameters.
func PostJSON[Resp any](
	ctx context.Context,
	r Requester,
	path string,
	reqMsg any,
	mods ...aoni.RequestModifier,
) (*Resp, error) {
	var query url.Values
	if sid := r.SessionID(BaseURL); sid != "" {
		query = url.Values{"sessionid": {sid}}
	}

	var bodyBytes []byte
	if reqMsg != nil {
		var err error

		bodyBytes, err = json.Marshal(reqMsg)
		if err != nil {
			return nil, err
		}
	}

	allMods := make([]aoni.RequestModifier, 0, 4+len(mods))
	allMods = append(allMods, aoni.WithQuery(query))
	allMods = append(allMods, aoni.WithContentType("application/json; charset=UTF-8"))
	allMods = append(allMods, aoni.WithAccept("application/json"))
	allMods = append(allMods, aoni.WithDecoder(encoding.SteamJSONDecoder))
	allMods = append(allMods, mods...)

	return aoni.PostJSON[Resp](ctx, r, path, bytes.NewReader(bodyBytes), allMods...)
}

type decoratedRequester struct {
	Requester
	defaultMods []aoni.RequestModifier
}

func (d *decoratedRequester) Request(
	ctx context.Context,
	method, path string,
	mods ...aoni.RequestModifier,
) (*http.Response, error) {
	allMods := make([]aoni.RequestModifier, 0, len(d.defaultMods)+len(mods))
	allMods = append(allMods, d.defaultMods...)
	allMods = append(allMods, mods...)

	return d.Requester.Request(ctx, method, path, allMods...)
}
