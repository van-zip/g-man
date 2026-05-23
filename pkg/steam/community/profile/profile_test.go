// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package profile

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/lemon4ksan/g-man/pkg/rest"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
)

type mockRequester struct {
	mock.Mock
}

func (m *mockRequester) Request(
	ctx context.Context,
	method, path string,
	body, query any,
	mods ...rest.RequestModifier,
) (*http.Response, error) {
	args := m.Called(ctx, method, path, body, query, mods)

	var resp *http.Response
	if args.Get(0) != nil {
		resp = args.Get(0).(*http.Response)
	}

	return resp, args.Error(1)
}

func (m *mockRequester) SessionID(baseURL string) string {
	return m.Called(baseURL).String(0)
}

func TestEditProfile(t *testing.T) {
	ctx := context.Background()
	steamID := id.ID(76561197960265728)

	t.Run("Success with overrides", func(t *testing.T) {
		client := new(mockRequester)
		client.On("SessionID", mock.Anything).Return("mock_session_id")

		// 1. GET response html
		editHTML := `
		<html>
			<body>
				<div id="profile_edit_config" data-profile-edit='{"strPersonaName":"OldNickname","strRealName":"OldRealName","strSummary":"OldSummary","strCustomURL":"oldurl","LocationData":{"locCountryCode":"US","locStateCode":"FL","locCityCode":"Miami"}}'></div>
			</body>
		</html>`
		client.On("Request", mock.Anything, http.MethodGet, "profiles/76561197960265728/edit/info", mock.Anything, mock.Anything, mock.Anything).
			Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(editHTML)),
			}, nil).
			Once()

		// 2. POST save response
		client.On("Request", mock.Anything, http.MethodPost, "profiles/76561197960265728/edit", mock.Anything, mock.Anything, mock.Anything).
			Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"success":1}`)),
			}, nil).
			Once()

		newName := "NewNickname"
		newSummary := "NewSummary"
		newCountry := "CA"
		newCustomURL := "newurl"
		settings := Settings{
			Name:      &newName,
			Summary:   &newSummary,
			Country:   &newCountry,
			CustomURL: &newCustomURL,
		}

		err := EditProfile(ctx, client, steamID, settings)
		assert.NoError(t, err)

		// Assertions on the captured arguments
		var postCall *mock.Call
		for _, call := range client.Calls {
			if call.Method == "Request" && call.Arguments.String(1) == http.MethodPost {
				postCall = &call
				break
			}
		}

		assert.NotNil(t, postCall)
		postBody := postCall.Arguments.Get(3).([]byte)
		bodyStr := string(postBody)
		assert.Contains(t, bodyStr, "personaName=NewNickname")
		assert.Contains(t, bodyStr, "real_name=OldRealName")
		assert.Contains(t, bodyStr, "summary=NewSummary")
		assert.Contains(t, bodyStr, "country=CA")
		assert.Contains(t, bodyStr, "customURL=newurl")

		client.AssertExpectations(t)
	})

	t.Run("GET fails", func(t *testing.T) {
		client := new(mockRequester)
		client.On("Request", mock.Anything, http.MethodGet, "profiles/76561197960265728/edit/info", mock.Anything, mock.Anything, mock.Anything).
			Return(nil, errors.New("network error")).
			Once()

		err := EditProfile(ctx, client, steamID, Settings{})
		assert.ErrorContains(t, err, "network error")
	})

	t.Run("Missing config element", func(t *testing.T) {
		client := new(mockRequester)
		client.On("Request", mock.Anything, http.MethodGet, "profiles/76561197960265728/edit/info", mock.Anything, mock.Anything, mock.Anything).
			Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`<html><body></body></html>`)),
			}, nil).
			Once()

		err := EditProfile(ctx, client, steamID, Settings{})
		assert.ErrorContains(t, err, "could not find profile_edit_config element")
	})

	t.Run("Save failed response", func(t *testing.T) {
		client := new(mockRequester)
		client.On("SessionID", mock.Anything).Return("mock_session_id")

		editHTML := `<div id="profile_edit_config" data-profile-edit='{"strPersonaName":"a"}'></div>`
		client.On("Request", mock.Anything, http.MethodGet, "profiles/76561197960265728/edit/info", mock.Anything, mock.Anything, mock.Anything).
			Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(editHTML)),
			}, nil).
			Once()

		client.On("Request", mock.Anything, http.MethodPost, "profiles/76561197960265728/edit", mock.Anything, mock.Anything, mock.Anything).
			Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"success":0,"errmsg":"invalid custom URL"}`)),
			}, nil).
			Once()

		err := EditProfile(ctx, client, steamID, Settings{})
		assert.ErrorContains(t, err, "save failed: invalid custom URL")
	})
}

func TestUpdatePrivacySettings(t *testing.T) {
	ctx := context.Background()
	steamID := id.ID(76561197960265728)

	t.Run("Success privacy override", func(t *testing.T) {
		client := new(mockRequester)
		client.On("SessionID", mock.Anything).Return("mock_session_id")

		settingsHTML := `
		<html>
			<body>
				<div id="profile_edit_config" data-profile-edit='{"Privacy":{"PrivacySettings":{"PrivacyProfile":3,"PrivacyInventory":3,"PrivacyInventoryGifts":3,"PrivacyOwnedGames":3,"PrivacyPlaytime":3,"PrivacyFriendsList":3},"eCommentPermission":1}}'></div>
			</body>
		</html>`
		client.On("Request", mock.Anything, http.MethodGet, "profiles/76561197960265728/edit/settings", mock.Anything, mock.Anything, mock.Anything).
			Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(settingsHTML)),
			}, nil).
			Once()

		client.On("Request", mock.Anything, http.MethodPost, "profiles/76561197960265728/ajaxsetprivacy", mock.Anything, mock.Anything, mock.Anything).
			Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"success":1}`)),
			}, nil).
			Once()

		newProfile := PrivacyPrivate
		newComments := CommentPrivate
		giftsPrivate := true
		settings := PrivacySettings{
			Profile:        &newProfile,
			Comments:       &newComments,
			InventoryGifts: &giftsPrivate,
		}

		err := UpdatePrivacySettings(ctx, client, steamID, settings)
		assert.NoError(t, err)

		// Assertions on the captured arguments
		var postCall *mock.Call
		for _, call := range client.Calls {
			if call.Method == "Request" && call.Arguments.String(1) == http.MethodPost {
				postCall = &call
				break
			}
		}

		assert.NotNil(t, postCall)
		postBody := postCall.Arguments.Get(3).([]byte)
		bodyStr := string(postBody)
		assert.Contains(
			t,
			bodyStr,
			"Privacy=%7B%22PrivacyProfile%22%3A1%2C%22PrivacyInventory%22%3A3%2C%22PrivacyInventoryGifts%22%3A1%2C%22PrivacyOwnedGames%22%3A3%2C%22PrivacyPlaytime%22%3A3%2C%22PrivacyFriendsList%22%3A3%7D",
		)
		assert.Contains(t, bodyStr, "eCommentPermission=2") // CommentPrivate

		client.AssertExpectations(t)
	})

	t.Run("AJAX Set Privacy fails", func(t *testing.T) {
		client := new(mockRequester)
		client.On("SessionID", mock.Anything).Return("mock_session_id")

		settingsHTML := `<div id="profile_edit_config" data-profile-edit='{"Privacy":{"PrivacySettings":{},"eCommentPermission":1}}'></div>`
		client.On("Request", mock.Anything, http.MethodGet, "profiles/76561197960265728/edit/settings", mock.Anything, mock.Anything, mock.Anything).
			Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(settingsHTML)),
			}, nil).
			Once()

		client.On("Request", mock.Anything, http.MethodPost, "profiles/76561197960265728/ajaxsetprivacy", mock.Anything, mock.Anything, mock.Anything).
			Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"success":0}`)),
			}, nil).
			Once()

		err := UpdatePrivacySettings(ctx, client, steamID, PrivacySettings{})
		assert.ErrorContains(t, err, "privacy save failed: success=0")
	})
}

func TestUploadAvatar(t *testing.T) {
	ctx := context.Background()
	steamID := id.ID(76561197960265728)
	dummyImage := []byte("image_data_bytes_12345")

	t.Run("Success jpg upload", func(t *testing.T) {
		client := new(mockRequester)
		client.On("SessionID", mock.Anything).Return("mock_session_id")

		client.On("Request", mock.Anything, http.MethodPost, "actions/FileUploader", mock.Anything, mock.Anything, mock.Anything).
			Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"success":true,"hash":"new_jpg_avatar_hash"}`)),
			}, nil).
			Once()

		hash, err := UploadAvatar(ctx, client, steamID, dummyImage, "image/jpeg")
		assert.NoError(t, err)
		assert.Equal(t, "new_jpg_avatar_hash", hash)

		// Assertions on the captured arguments
		var postCall *mock.Call
		for _, call := range client.Calls {
			if call.Method == "Request" && call.Arguments.String(1) == http.MethodPost {
				postCall = &call
				break
			}
		}

		assert.NotNil(t, postCall)
		postBody := postCall.Arguments.Get(3).([]byte)
		bodyStr := string(postBody)
		assert.Contains(t, bodyStr, "player_avatar_image")
		assert.Contains(t, bodyStr, "76561197960265728")
		assert.Contains(t, bodyStr, "avatar.jpg")
		assert.Contains(t, bodyStr, "image_data_bytes_12345")

		client.AssertExpectations(t)
	})

	t.Run("Empty image", func(t *testing.T) {
		client := new(mockRequester)
		_, err := UploadAvatar(ctx, client, steamID, nil, "png")
		assert.ErrorContains(t, err, "empty avatar image buffer")
	})

	t.Run("Unsupported format", func(t *testing.T) {
		client := new(mockRequester)
		_, err := UploadAvatar(ctx, client, steamID, dummyImage, "image/tiff")
		assert.ErrorContains(t, err, "unsupported content-type")
	})

	t.Run("Upload fails with error message", func(t *testing.T) {
		client := new(mockRequester)
		client.On("SessionID", mock.Anything).Return("mock_session_id")

		client.On("Request", mock.Anything, http.MethodPost, "actions/FileUploader", mock.Anything, mock.Anything, mock.Anything).
			Return(&http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBufferString(`{"success":false,"message":"file too large"}`)),
			}, nil).
			Once()

		_, err := UploadAvatar(ctx, client, steamID, dummyImage, "png")
		assert.ErrorContains(t, err, "upload failed: file too large")
	})
}
