// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"github.com/lemon4ksan/g-man/pkg/trading/notifications"
	"github.com/lemon4ksan/g-man/pkg/trading/reason"
)

func init() {
	// Register TF2 specific notification templates.
	// This keeps the core trading package game-agnostic.
	notifications.RegisterDefaultTemplate(
		"decline."+reason.DeclineNonTF2.String(),
		"/pre ❌ This bot only trades TF2 items. Your offer was declined.",
	)
	notifications.RegisterDefaultTemplate(
		"decline."+reason.DeclineCrimeAttempt.String(),
		"/pre ❌ Your offer was declined for attempting to take items for free.",
	)
}
