// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package reason contains the possible trade failure reasons for processing.
package reason

// TradeReason represents a unique string identifier explaining a trade reasoning decision.
type TradeReason string

// Inventory and generic trade reasons.
const (
	// ReviewInvalidItems indicates that the offer contains items not present in our pricelist.
	ReviewInvalidItems TradeReason = "🟨_INVALID_ITEMS"
	// ReviewDisabledItems indicates that the offer contains items currently disabled for trading.
	ReviewDisabledItems TradeReason = "🟧_DISABLED_ITEMS"
	// ReviewOverstocked indicates that accepting the offer would exceed our stock limits.
	ReviewOverstocked TradeReason = "🟦_OVERSTOCKED"
	// ReviewUnderstocked indicates that we lack the items requested in the trade.
	ReviewUnderstocked TradeReason = "🟩_UNDERSTOCKED"
	// ReviewBannedCheckFailed indicates that community ban check verification failed.
	ReviewBannedCheckFailed TradeReason = "⬜_BANNED_CHECK_FAILED"
	// ReviewEscrowCheckFailed indicates that escrow duration verification failed.
	ReviewEscrowCheckFailed TradeReason = "⬜_ESCROW_CHECK_FAILED"
	// ReviewHalted indicates that trading is currently paused.
	ReviewHalted TradeReason = "⬜_HALTED"
	// ReviewReviewForced indicates that a manual review was forced.
	ReviewReviewForced TradeReason = "⬜_REVIEW_FORCED"
	// ReviewEngineError indicates that an internal reasoning engine error occurred.
	ReviewEngineError TradeReason = "⬜_ENGINE_ERROR"
	// ReviewPartnerInventoryFetchFailed indicates that we failed to fetch the partner's inventory.
	ReviewPartnerInventoryFetchFailed TradeReason = "⬜_PARTNER_INVENTORY_FETCH_FAILED"
	// DeclineManual indicates that the trade was manually declined.
	DeclineManual TradeReason = "MANUAL"
	// DeclineHalted indicates that the trade was declined because trading is paused.
	DeclineHalted TradeReason = "HALTED"
	// DeclineEscrow indicates that the trade was declined because it would result in escrow.
	DeclineEscrow TradeReason = "ESCROW"
	// DeclineBanned indicates that the partner is banned in one or more communities.
	DeclineBanned TradeReason = "BANNED"
	// DeclineBlacklisted indicates that the partner is blacklisted.
	DeclineBlacklisted TradeReason = "BLACKLISTED"
	// DeclineOverstocked indicates that we are overstocked on the offered items.
	DeclineOverstocked TradeReason = "OVERSTOCKED"
	// DeclineBegging indicates that the partner is asking for items for free.
	DeclineBegging TradeReason = "BEGGING"

	// DeclineInternalError indicates that an internal processing error occurred.
	DeclineInternalError TradeReason = "INTERNAL_ERROR"
	// DeclineJunkDonation indicates that the offered donation consists entirely of untradable junk.
	DeclineJunkDonation TradeReason = "JUNK_DONATION"

	// AcceptDonation indicates that the offer is accepted as a donation.
	AcceptDonation TradeReason = "DONATION"
	// AcceptCorrectValue indicates that the offer contains balanced, correct values.
	AcceptCorrectValue TradeReason = "CORRECT_VALUE"
)

// String returns the string representation of the trade reason.
func (r TradeReason) String() string {
	return string(r)
}
