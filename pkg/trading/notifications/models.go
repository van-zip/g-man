// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package notifications

import (
	"context"

	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/trading/reason"
)

// TradeState represents the transactional state of a trade offer.
type TradeState int

const (
	// StateInvalid represents a malformed or invalid offer state.
	StateInvalid TradeState = iota
	// StateActive represents an active offer awaiting response.
	StateActive
	// StateAccepted represents an offer that has been accepted and finalized.
	StateAccepted
	// StateCountered represents an offer that was countered with modified parameters.
	StateCountered
	// StateExpired represents an offer that passed its expiration threshold.
	StateExpired
	// StateCanceled represents an offer initiated by us that we withdrew.
	StateCanceled
	// StateDeclined represents an offer that we declined.
	StateDeclined
	// StateInvalidItems represents an offer that failed because items are no longer available.
	StateInvalidItems
	// StateCreatedNeedsConfirmation represents an offer that is pending mobile app confirmation.
	StateCreatedNeedsConfirmation
	// StateCanceledBySecondFactor represents an offer canceled due to a failed mobile confirmation.
	StateCanceledBySecondFactor
	// StateInEscrow represents an offer accepted but currently held in Steam escrow.
	StateInEscrow
)

// TradeInfo contains the detailed metadata required to populate notification templates.
type TradeInfo struct {
	// OfferID is the unique transaction identifier assigned by Steam.
	OfferID uint64
	// PartnerSteamID is the 64-bit Steam ID of the trade partner.
	PartnerSteamID id.ID
	// ReasonType is the specific reasoning decision triggered by the offer.
	ReasonType reason.TradeReason
	// OldState is the previous transactional state of the trade offer.
	OldState TradeState
	// IsCanceledByUser is true if the offer was canceled by the initiating user.
	IsCanceledByUser bool
	// BannedStatus holds the community ban status checks for the partner.
	BannedStatus map[string]string
	// HighValueNames contains the names of high-value items involved in the trade.
	HighValueNames []string
	// MissingValue is the text representation of value discrepancies (e.g. "1.33 ref").
	MissingValue string
}

// ConfigProvider defines the contract for retrieving customized notification templates and prefixes.
type ConfigProvider interface {
	// GetTemplate retrieves the template string associated with a specific key.
	GetTemplate(key string) string
	// GetCommandPrefix retrieves the chat command prefix (such as "!").
	GetCommandPrefix() string
}

// ChatProvider defines the interface for sending messages to a Steam user.
type ChatProvider interface {
	// SendMessage transmits a text message to a specific Steam user ID.
	SendMessage(ctx context.Context, steamID id.ID, message string) error
}
