// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package crypto

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"regexp"
)

const (
	steamChars = "23456789BCDFGHJKMNPQRTVWXY"
)

var secretRegex = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

// GenerateAuthCode generates a 5-digit Steam Guard two-factor authentication code for the given timestamp.
//
// The sharedSecret argument must be a base64-encoded or 40-character hex-encoded string.
//
// It returns an error if the shared secret cannot be decoded from base64 or hexadecimal.
func GenerateAuthCode(sharedSecret string, timestamp int64) (string, error) {
	secret, err := decodeSecret(sharedSecret)
	if err != nil {
		return "", err
	}

	// Steam Guard uses 30-second windows
	t := uint64(timestamp / 30)

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, t)

	mac := hmac.New(sha1.New, secret)
	mac.Write(buf)
	sum := mac.Sum(nil)

	start := sum[19] & 0x0F
	fullCode := binary.BigEndian.Uint32(sum[start:start+4]) & 0x7FFFFFFF

	code := make([]byte, 5)
	for i := range 5 {
		code[i] = steamChars[fullCode%uint32(len(steamChars))]
		fullCode /= uint32(len(steamChars))
	}

	return string(code), nil
}

// GenerateConfirmationKey generates a base64-encoded key required to confirm mobile actions.
//
// The identitySecret must be a base64-encoded or 40-character hex-encoded string.
// The tag parameter represents the action type (such as "conf", "allow", or "cancel").
// If the tag exceeds 32 bytes, only the first 32 bytes of the tag are used.
//
// It returns an error if the identity secret cannot be decoded from base64 or hexadecimal.
func GenerateConfirmationKey(identitySecret string, timestamp int64, tag string) (string, error) {
	secret, err := decodeSecret(identitySecret)
	if err != nil {
		return "", err
	}

	dataLen := 8
	if len(tag) > 32 {
		dataLen += 32
	} else {
		dataLen += len(tag)
	}

	buf := make([]byte, dataLen)
	binary.BigEndian.PutUint64(buf, uint64(timestamp))
	copy(buf[8:], []byte(tag))

	mac := hmac.New(sha1.New, secret)
	mac.Write(buf)

	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

// GetDeviceID generates a unique, deterministic device identifier based on the SteamID.
// It returns a formatted UUID string with an "android:" prefix.
func GetDeviceID(steamID uint64) string {
	h := sha1.New()
	fmt.Fprintf(h, "%d", steamID)
	sum := hex.EncodeToString(h.Sum(nil))

	// UUID: 8-4-4-4-12
	return fmt.Sprintf("android:%s-%s-%s-%s-%s",
		sum[:8],
		sum[8:12],
		sum[12:16],
		sum[16:20],
		sum[20:32],
	)
}

func decodeSecret(secret string) ([]byte, error) {
	if secretRegex.MatchString(secret) {
		return hex.DecodeString(secret)
	}

	return base64.StdEncoding.DecodeString(secret)
}
