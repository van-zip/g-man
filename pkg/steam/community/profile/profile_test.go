// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package profile

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/test/mock"
)

func TestEditProfile(t *testing.T) {
	ctx := context.Background()
	steamID := id.ID(76561197960265728)

	t.Run("Success with overrides", func(t *testing.T) {
		clientMock := mock.NewHTTPStub()

		editHTML := `
		<html>
			<body>
				<div id="profile_edit_config" data-profile-edit='{"strPersonaName":"OldNickname","strRealName":"OldRealName","strSummary":"OldSummary","strCustomURL":"oldurl","LocationData":{"locCountryCode":"US","locStateCode":"FL","locCityCode":"Miami"}}'></div>
			</body>
		</html>`
		clientMock.SetHTMLResponse("profiles/{steamID}/edit/info", 200, editHTML)
		clientMock.SetJSONResponse("profiles/{steamID}/edit", 200, map[string]int{"success": 1})

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

		err := EditProfile(ctx, clientMock, steamID, settings)
		assert.NoError(t, err)

		lastCall := clientMock.GetLastCall()
		require.NotNil(t, lastCall)

		bodyBytes, _ := io.ReadAll(lastCall.Body)
		bodyStr := string(bodyBytes)
		assert.Contains(t, bodyStr, "personaName=NewNickname")
		assert.Contains(t, bodyStr, "real_name=OldRealName")
		assert.Contains(t, bodyStr, "summary=NewSummary")
		assert.Contains(t, bodyStr, "country=CA")
		assert.Contains(t, bodyStr, "customURL=newurl")
	})

	t.Run("Missing config element", func(t *testing.T) {
		clientMock := mock.NewHTTPStub()
		clientMock.SetHTMLResponse("profiles/{steamID}/edit/info", 200, `<html><body></body></html>`)

		err := EditProfile(ctx, clientMock, steamID, Settings{})
		assert.ErrorContains(t, err, "could not find profile_edit_config element")
	})

	t.Run("Save failed response", func(t *testing.T) {
		clientMock := mock.NewHTTPStub()

		editHTML := `<div id="profile_edit_config" data-profile-edit='{"strPersonaName":"a"}'></div>`
		clientMock.SetHTMLResponse("profiles/{steamID}/edit/info", 200, editHTML)
		clientMock.SetJSONResponse(
			"profiles/{steamID}/edit",
			200,
			map[string]any{"success": 0, "errmsg": "invalid custom URL"},
		)

		err := EditProfile(ctx, clientMock, steamID, Settings{})
		assert.ErrorContains(t, err, "save failed: invalid custom URL")
	})
}

func TestUpdatePrivacySettings(t *testing.T) {
	ctx := context.Background()
	steamID := id.ID(76561197960265728)

	t.Run("Success privacy override", func(t *testing.T) {
		clientMock := mock.NewHTTPStub()

		settingsHTML := `
		<html>
			<body>
				<div id="profile_edit_config" data-profile-edit='{"Privacy":{"PrivacySettings":{"PrivacyProfile":3,"PrivacyInventory":3,"PrivacyInventoryGifts":3,"PrivacyOwnedGames":3,"PrivacyPlaytime":3,"PrivacyFriendsList":3},"eCommentPermission":1}}'></div>
			</body>
		</html>`
		clientMock.SetHTMLResponse("profiles/{steamID}/edit/settings", 200, settingsHTML)
		clientMock.SetJSONResponse("profiles/{steamID}/ajaxsetprivacy", 200, map[string]int{"success": 1})

		newProfile := PrivacyPrivate
		newComments := CommentPrivate
		giftsPrivate := true
		settings := PrivacySettings{
			Profile:        &newProfile,
			Comments:       &newComments,
			InventoryGifts: &giftsPrivate,
		}

		err := UpdatePrivacySettings(ctx, clientMock, steamID, settings)
		assert.NoError(t, err)

		lastCall := clientMock.GetLastCall()
		require.NotNil(t, lastCall)

		bodyBytes, _ := io.ReadAll(lastCall.Body)
		bodyStr := string(bodyBytes)
		assert.Contains(
			t,
			bodyStr,
			"Privacy=%7B%22PrivacyProfile%22%3A1%2C%22PrivacyInventory%22%3A3%2C%22PrivacyInventoryGifts%22%3A1%2C%22PrivacyOwnedGames%22%3A3%2C%22PrivacyPlaytime%22%3A3%2C%22PrivacyFriendsList%22%3A3%7D",
		)
		assert.Contains(t, bodyStr, "eCommentPermission=2") // CommentPrivate
	})

	t.Run("AJAX Set Privacy fails", func(t *testing.T) {
		clientMock := mock.NewHTTPStub()

		settingsHTML := `<div id="profile_edit_config" data-profile-edit='{"Privacy":{"PrivacySettings":{},"eCommentPermission":1}}'></div>`
		clientMock.SetHTMLResponse("profiles/{steamID}/edit/settings", 200, settingsHTML)
		clientMock.SetJSONResponse("profiles/{steamID}/ajaxsetprivacy", 200, map[string]int{"success": 0})

		err := UpdatePrivacySettings(ctx, clientMock, steamID, PrivacySettings{})
		assert.ErrorContains(t, err, "privacy save failed: success=0")
	})
}

func TestUploadAvatar(t *testing.T) {
	ctx := context.Background()
	steamID := id.ID(76561197960265728)
	dummyImage := []byte("image_data_bytes_12345")

	t.Run("Success jpg upload", func(t *testing.T) {
		clientMock := mock.NewHTTPStub()
		clientMock.SetJSONResponse(
			"actions/FileUploader",
			200,
			map[string]any{"success": true, "hash": "new_jpg_avatar_hash"},
		)

		hash, err := UploadAvatar(ctx, clientMock, steamID, dummyImage, "image/jpeg")
		assert.NoError(t, err)
		assert.Equal(t, "new_jpg_avatar_hash", hash)

		lastCall := clientMock.GetLastCall()
		require.NotNil(t, lastCall)

		bodyBytes, _ := io.ReadAll(lastCall.Body)
		bodyStr := string(bodyBytes)
		assert.Contains(t, bodyStr, "player_avatar_image")
		assert.Contains(t, bodyStr, "76561197960265728")
		assert.Contains(t, bodyStr, "avatar.jpg")
		assert.Contains(t, bodyStr, "image_data_bytes_12345")
	})

	t.Run("Empty image", func(t *testing.T) {
		clientMock := mock.NewHTTPStub()
		_, err := UploadAvatar(ctx, clientMock, steamID, nil, "png")
		assert.ErrorContains(t, err, "empty avatar image buffer")
	})

	t.Run("Unsupported format", func(t *testing.T) {
		clientMock := mock.NewHTTPStub()
		_, err := UploadAvatar(ctx, clientMock, steamID, dummyImage, "image/tiff")
		assert.ErrorContains(t, err, "unsupported content-type")
	})

	t.Run("Upload fails with error message", func(t *testing.T) {
		clientMock := mock.NewHTTPStub()
		clientMock.SetJSONResponse(
			"actions/FileUploader",
			200,
			map[string]any{"success": false, "message": "file too large"},
		)

		_, err := UploadAvatar(ctx, clientMock, steamID, dummyImage, "png")
		assert.ErrorContains(t, err, "upload failed: file too large")
	})
}
