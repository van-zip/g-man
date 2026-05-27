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
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"

	"github.com/lemon4ksan/g-man/pkg/steam/community"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
)

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
	path := fmt.Sprintf("profiles/%d/edit/info", steamID)

	htmlBytes, err := community.GetHTML(ctx, client, path)
	if err != nil {
		return fmt.Errorf("profile: failed to fetch edit page: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlBytes))
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

	form := url.Values{
		"sessionID":       {client.SessionID(community.BaseURL)},
		"type":            {"profileSave"},
		"weblink_1_title": {""},
		"weblink_1_url":   {""},
		"weblink_2_title": {""},
		"weblink_2_url":   {""},
		"weblink_3_title": {""},
		"weblink_3_url":   {""},
		"personaName":     {name},
		"real_name":       {realName},
		"summary":         {summary},
		"country":         {country},
		"state":           {state},
		"city":            {city},
		"customURL":       {customURL},
		"json":            {"1"},
	}

	savePath := fmt.Sprintf("profiles/%d/edit", steamID)

	type saveResponse struct {
		Success int    `json:"success"`
		ErrMsg  string `json:"errmsg"`
	}

	resp, err := community.PostForm[saveResponse](ctx, client, savePath, form)
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
	path := fmt.Sprintf("profiles/%d/edit/settings", steamID)

	htmlBytes, err := community.GetHTML(ctx, client, path)
	if err != nil {
		return fmt.Errorf("profile: failed to fetch settings page: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(htmlBytes))
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

	form := url.Values{
		"sessionid":          {client.SessionID(community.BaseURL)},
		"Privacy":            {string(privacyJSON)},
		"eCommentPermission": {strconv.Itoa(comments)},
	}

	savePath := fmt.Sprintf("profiles/%d/ajaxsetprivacy", steamID)

	type privacyResponse struct {
		Success int `json:"success"`
	}

	resp, err := community.PostForm[privacyResponse](ctx, client, savePath, form)
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

	var body bytes.Buffer

	writer := multipart.NewWriter(&body)

	_ = writer.WriteField("MAX_FILE_SIZE", strconv.Itoa(len(image)))
	_ = writer.WriteField("type", "player_avatar_image")
	_ = writer.WriteField("sId", strconv.FormatUint(uint64(steamID), 10))
	_ = writer.WriteField("sessionid", client.SessionID(community.BaseURL))
	_ = writer.WriteField("doSub", "1")
	_ = writer.WriteField("json", "1")

	part, err := writer.CreateFormFile("avatar", filename)
	if err != nil {
		return "", fmt.Errorf("profile: failed to create form file: %w", err)
	}

	if _, err := part.Write(image); err != nil {
		return "", fmt.Errorf("profile: failed to write image bytes: %w", err)
	}

	_ = writer.Close()

	type uploadResponse struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Hash    string `json:"hash"`
	}

	// We utilize the underlying Requester directly to execute the POST with multipart/form-data
	resp, err := client.Request(
		ctx,
		http.MethodPost,
		"actions/FileUploader",
		body.Bytes(),
		nil,
		func(req *http.Request) {
			req.Header.Set("Content-Type", writer.FormDataContentType())
			req.Header.Set("Accept", "application/json, text/javascript; q=0.01")
		},
	)
	if err != nil {
		return "", fmt.Errorf("profile: upload request failed: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("profile: failed to read upload response: %w", err)
	}

	var result uploadResponse
	if err := json.Unmarshal(rawBody, &result); err != nil {
		return "", fmt.Errorf("profile: failed to unmarshal upload response: %w", err)
	}

	if !result.Success {
		errMsg := result.Message
		if errMsg == "" {
			errMsg = "upload was not successful"
		}

		return "", fmt.Errorf("profile: upload failed: %s", errMsg)
	}

	return result.Hash, nil
}
