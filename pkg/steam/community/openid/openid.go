// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package openid

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"

	"github.com/PuerkitoBio/goquery"
	"github.com/lemon4ksan/aoni"
)

var (
	// ErrNotSignedIn indicates that the provided Steam cookies are missing,
	// invalid, or expired, resulting in a redirect to the Steam login page.
	ErrNotSignedIn = errors.New("openid: not signed in to Steam (cookies expired or invalid)")

	// ErrNoForm indicates that the hidden OpenID submission form could not be found
	// on the Steam Community authorization page.
	ErrNoForm = errors.New("openid: could not find OpenID login form")

	// ErrWrongHost indicates that the initial request did not redirect to the
	// Steam Community OpenID provider as expected.
	ErrWrongHost = errors.New("openid: was not redirected to steamcommunity.com")
)

// Login performs an automated OpenID authorization flow on a third-party website
// using active Steam session cookies.
//
// The function returns a configured [aoni.Client] which contains a CookieJar populated
// with the target website's authorization cookies. This client can be used for
// subsequent API requests to the third-party service.
func Login(ctx context.Context, targetURL string, steamCookies []*http.Cookie) (*aoni.Client, error) {
	parsedTarget, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("openid: invalid target URL: %w", err)
	}

	steamCommURL, _ := url.Parse("https://steamcommunity.com")
	steamStoreURL, _ := url.Parse("https://store.steampowered.com")

	jar, _ := cookiejar.New(nil)
	jar.SetCookies(steamCommURL, steamCookies)
	jar.SetCookies(steamStoreURL, steamCookies)

	client := aoni.DefaultClient.WithCookieJar(jar)

	resp, err := client.Request(ctx, http.MethodGet, targetURL)
	if err != nil {
		return nil, fmt.Errorf("openid: initial request failed: %w", err)
	}
	defer resp.Body.Close()

	// Case 1: The site didn't redirect to Steam at all.
	if resp.Request.URL.Host == parsedTarget.Host {
		return client, nil
	}

	// Case 2: We were redirected, but not to where we expected.
	if resp.Request.URL.Host != "steamcommunity.com" {
		return nil, fmt.Errorf("%w: ended up at %s", ErrWrongHost, resp.Request.URL.Host)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("openid: failed to parse HTML: %w", err)
	}

	// Check if Steam is asking us to log in (meaning cookies are bad)
	if doc.Find("#loginForm").Length() > 0 {
		return nil, ErrNotSignedIn
	}

	form := doc.Find("#openidForm")
	if form.Length() == 0 {
		return nil, ErrNoForm
	}

	// Extract all hidden input fields from the form
	formData := url.Values{}
	form.Find("input").Each(func(i int, s *goquery.Selection) {
		value, _ := s.Attr("value")
		if name, exists := s.Attr("name"); exists && name != "" {
			formData.Set(name, value)
		}
	})

	// Emulates a "Sign In" button press
	if formData.Get("action") == "" {
		formData.Set("action", "steam_openid_login")
	}

	currentURL := resp.Request.URL
	postURL := "https://steamcommunity.com/openid/login"

	if action, exists := form.Attr("action"); exists && action != "" {
		if parsedAction, err := url.Parse(action); err == nil {
			postURL = currentURL.ResolveReference(parsedAction).String()
		}
	}

	_, err = aoni.PostForm[aoni.NoResponse](
		ctx, client, postURL, formData,
		aoni.WithHeader("Referer", currentURL.String()),
	)
	if err != nil {
		return nil, fmt.Errorf("openid: form submission failed: %w", err)
	}

	return client, nil
}
