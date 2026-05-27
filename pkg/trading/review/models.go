// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package review

import (
	"context"
	"slices"

	"github.com/lemon4ksan/g-man/pkg/trading/reason"
)

// BaseReason contains basic information about the reason for the hold.
type BaseReason struct {
	// Type specifies the unique trade reason identifier.
	Type reason.TradeReason
	// SKU is the stock-keeping unit of the problematic item.
	SKU string
}

// ReasonType returns the reason type.
func (b BaseReason) ReasonType() reason.TradeReason { return b.Type }

// ReasonOverstocked indicates that the offer would exceed our stock limits.
type ReasonOverstocked struct {
	BaseReason
	// AmountCanTrade is the maximum number of items we are willing to buy.
	AmountCanTrade int
	// AmountOffered is the number of items offered in the trade.
	AmountOffered int
}

// ReasonInvalidItems indicates that some items in the offer are not in our pricelist.
type ReasonInvalidItems struct {
	BaseReason
	// Price is the formatted target price representation.
	Price string
}

// ReasonDuped indicates that an item appears to be duplicated according to history.
type ReasonDuped struct {
	BaseReason
	// AssetID is the unique identifier of the duplicated item.
	AssetID string
}

// ReasonUnderstocked indicates that we don't have enough items to fulfill the trade.
type ReasonUnderstocked struct {
	BaseReason
	// AmountCanTrade is the number of items we currently have available.
	AmountCanTrade int
	// AmountTaking is the number of items requested in the trade.
	AmountTaking int
}

// ReasonInvalidValue indicates that the offer value is incorrect.
type ReasonInvalidValue struct {
	BaseReason
	// Diff is the absolute value difference.
	Diff float64
	// DiffRef is the value difference represented in refined metal.
	DiffRef float64
	// DiffKey is the value difference represented in keys.
	DiffKey string
}

// ReasonDisabledItems indicates that some items in the offer are currently disabled for trading.
type ReasonDisabledItems struct {
	BaseReason
}

// Meta contains summary information about the reasons for the trade hold.
type Meta struct {
	// UniqueReasons is the list of unique reason identifiers triggered.
	UniqueReasons []reason.TradeReason
	// Reasons contains detailed, struct-level descriptions of all triggered reasons.
	Reasons []interface{ ReasonType() reason.TradeReason }
}

// HasReason returns true if the meta contains the specified reason type.
func (m *Meta) HasReason(reasonType reason.TradeReason) bool {
	return slices.Contains(m.UniqueReasons, reasonType)
}

// Content contains generated texts for logs and chat.
type Content struct {
	Notes        []string
	ItemNamesOur map[string][]string // key: reason type, value: list of strings
	Missing      string
}

// SchemaProvider provides item name resolution from the schema.
type SchemaProvider interface {
	// GetName resolves an item's display name by its SKU or defindex.
	GetName(sku string, useDefindex bool) string
}

// ChatProvider handles sending messages to users and admins.
type ChatProvider interface {
	// SendMessage sends a text message to a specific Steam user.
	SendMessage(ctx context.Context, steamID uint64, message string) error
	// MessageAdmins sends a broadcast alert message to all administrators.
	MessageAdmins(ctx context.Context, message string) error
}

// PricelistProvider provides current key prices.
type PricelistProvider interface {
	// GetKeyPrices retrieves the current buy and sell prices in refined metal.
	GetKeyPrices() (buy, sell float64)
}

// ConfigProvider provides access to review-related configuration.
type ConfigProvider interface {
	// GetReviewTemplate retrieves the template string associated with a specific reason.
	GetReviewTemplate(reasonType reason.TradeReason) string
	// IsWebhookEnabled reports whether external webhook logging is enabled.
	IsWebhookEnabled() bool
}

// TradeMetadata stores offer metadata.
type TradeMetadata struct {
	// PrimaryReason is the main reason why the offer was held or declined.
	PrimaryReason reason.TradeReason
	// UniqueReasons is the list of all unique reasons triggered by the offer.
	UniqueReasons []string
	// Reasons contains structural detail payloads for each triggered reason.
	Reasons []interface{ ReasonType() reason.TradeReason }
	// BannedStatus holds the community ban status checks for the partner.
	BannedStatus map[string]string
	// HighValueNamesOur is the list of high-value items we are giving.
	HighValueNamesOur []string
	// HighValueNamesTheir is the list of high-value items we are receiving.
	HighValueNamesTheir []string
	// ProcessTimeMS is the time taken by the reasoning engine to process the offer in milliseconds.
	ProcessTimeMS int64
	// IsOfferSent is true if the offer was initiated and sent by our account.
	IsOfferSent bool
}

// DeclinedSummary stores formatted lists for output.
type DeclinedSummary struct {
	// ReasonDescription is the localized text describing the primary decline reason.
	ReasonDescription string
	// InvalidItems contains formatted descriptions of items not in the pricelist.
	InvalidItems []string
	// DisabledItems contains formatted descriptions of items currently disabled.
	DisabledItems []string
	// Overstocked contains formatted descriptions of items exceeding stock limits.
	Overstocked []string
	// Understocked contains formatted descriptions of items we lack.
	Understocked []string
	// DupedItems contains formatted descriptions of duplicated items.
	DupedItems []string
	// HighNotSellingItems contains formatted descriptions of high-value items not currently listed for sale.
	HighNotSellingItems []string
	// HighValue contains formatted descriptions of high-value items involved in the trade.
	HighValue []string
}

// BotStatsProvider provides various bot statistics.
type BotStatsProvider interface {
	// GetTotalItems retrieves the current total item count in the backpack.
	GetTotalItems() int
	// GetBackpackSlots retrieves the maximum backpack slot capacity.
	GetBackpackSlots() int
	// GetPureStock retrieves the current keys and refined metal stock levels.
	GetPureStock() (keys, ref float64)
	// GetVersion retrieves the current version string of the bot software.
	GetVersion() string
}

// AutokeysProvider provides status of the autokeys banking system.
type AutokeysProvider interface {
	// IsEnabled reports whether the autokeys banking system is enabled.
	IsEnabled() bool
	// IsActive reports whether the autokeys banking system is currently actively trading.
	IsActive() bool
	// GetStatus retrieves the current operational state string (such as "banking").
	GetStatus() string
}
