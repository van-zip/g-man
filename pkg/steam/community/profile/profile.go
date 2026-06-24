// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package profile

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/lemon4ksan/aoni"

	"github.com/lemon4ksan/g-man/pkg/steam/community"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
)

// WithAvatarUpload assembles a Steam-specific multipart avatar upload form.
// It ensures that the field name is "avatar" and the filename has the correct extension.
func WithAvatarUpload(fields map[string]string, filename string, image []byte) aoni.RequestModifier {
	return func(req *http.Request) {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		for k, v := range fields {
			if err := writer.WriteField(k, v); err != nil {
				return
			}
		}

		part, err := writer.CreateFormFile("avatar", filename)
		if err != nil {
			return
		}

		if _, err := part.Write(image); err != nil {
			return
		}

		if err := writer.Close(); err != nil {
			return
		}

		req.Body = io.NopCloser(body)
		req.ContentLength = int64(body.Len())
		req.Header.Set("Content-Type", writer.FormDataContentType())
	}
}

// Settings represents customizable profile details.
//
// Struct fields are pointers. Any field set to nil is ignored during update,
// keeping the existing profile values in Steam.
type Settings struct {
	// Name is the custom display nickname of the user.
	Name *string
	// RealName is the user's real name.
	RealName *string
	// Summary is the user's custom bio description.
	Summary *string
	// Country is the ISO two-letter country code of the location.
	Country *string
	// State is the state code of the location.
	State *string
	// City is the city code of the location.
	City *string
	// CustomURL is the custom vanity URL slug.
	CustomURL *string
}

// PrivacyState represents profile privacy level.
type PrivacyState int

// PrivacyState constants define the profile privacy level.
const (
	PrivacyPrivate     PrivacyState = 1
	PrivacyFriendsOnly PrivacyState = 2
	PrivacyPublic      PrivacyState = 3
)

// String returns a human-readable representation of PrivacyState.
func (p PrivacyState) String() string {
	switch p {
	case PrivacyPrivate:
		return "Private"
	case PrivacyFriendsOnly:
		return "FriendsOnly"
	case PrivacyPublic:
		return "Public"
	default:
		return "Unknown"
	}
}

// CommentPermission represents who can post comments on the profile.
type CommentPermission int

// CommentPermission constants define who can post comments on the profile.
const (
	CommentFriendsOnly CommentPermission = 0
	CommentAnyone      CommentPermission = 1
	CommentPrivate     CommentPermission = 2
)

// PrivacySettings represents customizable profile privacy details.
//
// Struct fields are pointers. Any field set to nil is ignored during update,
// keeping the existing privacy values in Steam.
type PrivacySettings struct {
	// Profile is the general privacy level of the profile.
	Profile *PrivacyState
	// Comments is the commentary permission level of the profile.
	Comments *CommentPermission
	// Inventory is the privacy level of the inventory.
	Inventory *PrivacyState
	// InventoryGifts specifies if inventory gifts are kept private (true) or public (false).
	InventoryGifts *bool
	// GameDetails is the privacy level of active game playtimes and details.
	GameDetails *PrivacyState
	// Playtime specifies if total gameplay playtime is kept private (true) or public (false).
	Playtime *bool
	// FriendsList is the privacy level of the friends list.
	FriendsList *PrivacyState
}

// EditProfile retrieves the existing profile configuration, merges the changes, and saves the updated settings.
//
// It returns an error if the request fails, if the edit page cannot be parsed,
// or if Steam rejects the updated parameters with an error description.
func EditProfile(ctx context.Context, client community.Requester, steamID id.ID, settings Settings) error {
	html, err := community.GetHTML(
		ctx, client, "profiles/{steamID}/edit/info",
		aoni.WithVar("steamID", steamID),
	)
	if err != nil {
		return fmt.Errorf("profile: failed to fetch edit page: %w", err)
	}
	defer html.Close()

	doc, err := goquery.NewDocumentFromReader(html)
	if err != nil {
		return fmt.Errorf("profile: failed to parse HTML: %w", err)
	}

	configEl := doc.Find("#profile_edit_config")
	if configEl.Length() == 0 {
		return errors.New("profile: could not find profile_edit_config element")
	}

	dataVal, exists := configEl.Attr("data-profile-edit")
	if !exists {
		return errors.New("profile: missing data-profile-edit attribute")
	}

	var existing rawProfileEditConfig
	if err := json.Unmarshal([]byte(dataVal), &existing); err != nil {
		return fmt.Errorf("profile: failed to parse existing settings JSON: %w", err)
	}

	// Merge overrides
	name := existing.PersonaName
	if settings.Name != nil {
		name = *settings.Name
	}

	realName := existing.RealName
	if settings.RealName != nil {
		realName = *settings.RealName
	}

	summary := existing.Summary
	if settings.Summary != nil {
		summary = *settings.Summary
	}

	country := existing.LocationData.CountryCode
	if settings.Country != nil {
		country = *settings.Country
	}

	state := existing.LocationData.StateCode
	if settings.State != nil {
		state = *settings.State
	}

	city := existing.LocationData.CityCode
	if settings.City != nil {
		city = *settings.City
	}

	customURL := existing.CustomURL
	if settings.CustomURL != nil {
		customURL = *settings.CustomURL
	}

	req := struct {
		Type          string `url:"type"`
		Weblink1Title string `url:"weblink_1_title"`
		Weblink1URL   string `url:"weblink_1_url"`
		Weblink2Title string `url:"weblink_2_title"`
		Weblink2URL   string `url:"weblink_2_url"`
		Weblink3Title string `url:"weblink_3_title"`
		Weblink3URL   string `url:"weblink_3_url"`
		PersonaName   string `url:"personaName"`
		RealName      string `url:"real_name"`
		Summary       string `url:"summary"`
		Country       string `url:"country"`
		State         string `url:"state"`
		City          string `url:"city"`
		CustomURL     string `url:"customURL"`
		JSON          int    `url:"json"`
	}{
		Type:        "profileSave",
		PersonaName: name,
		RealName:    realName,
		Summary:     summary,
		Country:     country,
		State:       state,
		City:        city,
		CustomURL:   customURL,
		JSON:        1,
	}

	type saveResponse struct {
		Success int    `json:"success"`
		ErrMsg  string `json:"errmsg"`
	}

	resp, err := community.PostForm[saveResponse](
		ctx, client, "profiles/{steamID}/edit", req,
		aoni.WithVar("steamID", steamID),
	)
	if err != nil {
		return fmt.Errorf("profile: failed to post profile save: %w", err)
	}

	if resp.Success != 1 {
		errMsg := resp.ErrMsg
		if errMsg == "" {
			errMsg = "request was not successful"
		}

		return fmt.Errorf("profile: save failed: %s", errMsg)
	}

	return nil
}

// UpdatePrivacySettings fetches the current privacy status, merges the changes, and posts updates.
//
// It returns an error if the request fails, if the settings page cannot be parsed,
// or if the update request is rejected by Steam.
func UpdatePrivacySettings(
	ctx context.Context,
	client community.Requester,
	steamID id.ID,
	settings PrivacySettings,
) error {
	html, err := community.GetHTML(
		ctx, client, "profiles/{steamID}/edit/settings",
		aoni.WithVar("steamID", steamID),
	)
	if err != nil {
		return fmt.Errorf("profile: failed to fetch settings page: %w", err)
	}
	defer html.Close()

	doc, err := goquery.NewDocumentFromReader(html)
	if err != nil {
		return fmt.Errorf("profile: failed to parse HTML: %w", err)
	}

	configEl := doc.Find("#profile_edit_config")
	if configEl.Length() == 0 {
		return errors.New("profile: could not find profile_edit_config element")
	}

	dataVal, exists := configEl.Attr("data-profile-edit")
	if !exists {
		return errors.New("profile: missing data-profile-edit attribute")
	}

	var existing rawPrivacyConfig
	if err := json.Unmarshal([]byte(dataVal), &existing); err != nil {
		return fmt.Errorf("profile: failed to parse existing privacy settings JSON: %w", err)
	}

	commentMapping := map[CommentPermission]int{
		CommentFriendsOnly: 0,
		CommentAnyone:      1,
		CommentPrivate:     2,
	}

	privacy := existing.Privacy.PrivacySettings
	comments := existing.Privacy.ECommentPermission

	if settings.Profile != nil {
		privacy.PrivacyProfile = int(*settings.Profile)
	}

	if settings.Comments != nil {
		comments = commentMapping[*settings.Comments]
	}

	if settings.Inventory != nil {
		privacy.PrivacyInventory = int(*settings.Inventory)
	}

	if settings.InventoryGifts != nil {
		if *settings.InventoryGifts {
			privacy.PrivacyInventoryGifts = int(PrivacyPrivate)
		} else {
			privacy.PrivacyInventoryGifts = int(PrivacyPublic)
		}
	}

	if settings.GameDetails != nil {
		privacy.PrivacyOwnedGames = int(*settings.GameDetails)
	}

	if settings.Playtime != nil {
		if *settings.Playtime {
			privacy.PrivacyPlaytime = int(PrivacyPrivate)
		} else {
			privacy.PrivacyPlaytime = int(PrivacyPublic)
		}
	}

	if settings.FriendsList != nil {
		privacy.PrivacyFriendsList = int(*settings.FriendsList)
	}

	privacyJSON, err := json.Marshal(privacy)
	if err != nil {
		return fmt.Errorf("profile: failed to marshal privacy settings: %w", err)
	}

	form := struct {
		Privacy            string `url:"Privacy"`
		ECommentPermission int    `url:"eCommentPermission"`
	}{string(privacyJSON), comments}

	type privacyResponse struct {
		Success int `json:"success"`
	}

	resp, err := community.PostForm[privacyResponse](
		ctx, client, "profiles/{steamID}/ajaxsetprivacy", form,
		aoni.WithVar("steamID", steamID),
	)
	if err != nil {
		return fmt.Errorf("profile: failed to post privacy settings: %w", err)
	}

	if resp.Success != 1 {
		return fmt.Errorf("profile: privacy save failed: success=%d", resp.Success)
	}

	return nil
}

// UploadAvatar uploads a new profile avatar image and returns its secure hash.
//
// It accepts "image/jpeg", "image/png", and "image/gif" content types.
// It returns an error if the image buffer is empty, if the content type is unsupported,
// if the multipart form cannot be constructed, or if the upload is rejected.
func UploadAvatar(
	ctx context.Context,
	client community.Requester,
	steamID id.ID,
	image []byte,
	contentType string,
) (string, error) {
	if len(image) == 0 {
		return "", errors.New("profile: empty avatar image buffer")
	}

	var filename string
	switch strings.ToLower(contentType) {
	case "image/jpeg", "image/jpg", "jpg", "jpeg":
		filename = "avatar.jpg"
	case "image/png", "png":
		filename = "avatar.png"
	case "image/gif", "gif":
		filename = "avatar.gif"
	default:
		return "", fmt.Errorf("profile: unsupported content-type: %s", contentType)
	}

	fields := map[string]string{
		"MAX_FILE_SIZE": strconv.Itoa(len(image)),
		"type":          "player_avatar_image",
		"sId":           strconv.FormatUint(uint64(steamID), 10),
		"sessionid":     client.SessionID(community.BaseURL),
		"doSub":         "1",
		"json":          "1",
	}

	type upload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Hash    string `json:"hash"`
	}

	resp, err := aoni.PostJSON[upload](
		ctx, client, "actions/FileUploader", nil,
		WithAvatarUpload(fields, filename, image),
		aoni.WithHeader("Accept", "application/json, text/javascript; q=0.01"),
	)
	if err != nil {
		return "", fmt.Errorf("profile: upload request failed: %w", err)
	}

	if !resp.Success {
		errMsg := resp.Message
		if errMsg == "" {
			errMsg = "upload was not successful"
		}

		return "", fmt.Errorf("profile: upload failed: %s", errMsg)
	}

	return resp.Hash, nil
}
