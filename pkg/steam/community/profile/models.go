// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package profile

type rawProfileLocation struct {
	CountryCode string `json:"locCountryCode"`
	StateCode   string `json:"locStateCode"`
	CityCode    string `json:"locCityCode"`
}

type rawProfileEditConfig struct {
	PersonaName  string             `json:"strPersonaName"`
	RealName     string             `json:"strRealName"`
	Summary      string             `json:"strSummary"`
	CustomURL    string             `json:"strCustomURL"`
	LocationData rawProfileLocation `json:"LocationData"`
}

type rawPrivacySettings struct {
	PrivacyProfile        int `json:"PrivacyProfile"`
	PrivacyInventory      int `json:"PrivacyInventory"`
	PrivacyInventoryGifts int `json:"PrivacyInventoryGifts"`
	PrivacyOwnedGames     int `json:"PrivacyOwnedGames"`
	PrivacyPlaytime       int `json:"PrivacyPlaytime"`
	PrivacyFriendsList    int `json:"PrivacyFriendsList"`
}

type rawPrivacyData struct {
	PrivacySettings    rawPrivacySettings `json:"PrivacySettings"`
	ECommentPermission int                `json:"eCommentPermission"`
}

type rawPrivacyConfig struct {
	Privacy rawPrivacyData `json:"Privacy"`
}
