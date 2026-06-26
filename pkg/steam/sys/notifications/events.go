// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package notifications

import (
	"fmt"

	"github.com/lemon4ksan/miyako/bus"

	pb "github.com/lemon4ksan/g-man/pkg/protobuf/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
)

// NotificationType represents the type of a Steam user notification.
type NotificationType uint32

const (
	// NotificationInvalid is an invalid/uninitialized notification type.
	NotificationInvalid NotificationType = 0
	// NotificationTest is a test notification.
	NotificationTest NotificationType = 1
	// NotificationGift indicates a new gift (game or guest pass) was received.
	NotificationGift NotificationType = 2
	// NotificationComment indicates a new comment (profile, screenshot, review, etc.).
	NotificationComment NotificationType = 3
	// NotificationItem indicates new items in the Steam inventory (cards, backgrounds, drops).
	NotificationItem NotificationType = 4
	// NotificationFriendInvite indicates a friend request.
	NotificationFriendInvite NotificationType = 5
	// NotificationMajorSale indicates a major Steam sale event.
	NotificationMajorSale NotificationType = 6
	// NotificationPreloadAvailable indicates preload is available for a purchased game.
	NotificationPreloadAvailable NotificationType = 7
	// NotificationWishlist indicates a wishlisted game released or went on sale.
	NotificationWishlist NotificationType = 8
	// NotificationTradeOffer indicates a new or changed trade offer.
	NotificationTradeOffer NotificationType = 9
	// NotificationGeneral is a general Steam notification.
	NotificationGeneral NotificationType = 10
	// NotificationHelpRequest indicates a Steam Support response.
	NotificationHelpRequest NotificationType = 11
	// NotificationAsyncGame indicates a turn in an async game.
	NotificationAsyncGame NotificationType = 12
	// NotificationChatMsg indicates a new chat message.
	NotificationChatMsg NotificationType = 13
	// NotificationModeratorMsg indicates a community moderator message.
	NotificationModeratorMsg NotificationType = 14
	// NotificationParentalFeatureAccessRequest indicates a child requests feature access.
	NotificationParentalFeatureAccessRequest NotificationType = 15
	// NotificationFamilyInvite indicates an invite to join a Steam Family.
	NotificationFamilyInvite NotificationType = 16
	// NotificationFamilyPurchaseRequest indicates a child requests a game purchase.
	NotificationFamilyPurchaseRequest NotificationType = 17
	// NotificationParentalPlaytimeRequest indicates a child requests more playtime.
	NotificationParentalPlaytimeRequest NotificationType = 18
	// NotificationFamilyPurchaseRequestResponse indicates a parent responded to a purchase request.
	NotificationFamilyPurchaseRequestResponse NotificationType = 19
	// NotificationParentalFeatureAccessResponse indicates a parent responded to a feature access request.
	NotificationParentalFeatureAccessResponse NotificationType = 20
	// NotificationParentalPlaytimeResponse indicates a parent responded to a playtime request.
	NotificationParentalPlaytimeResponse NotificationType = 21
	// NotificationRequestedGameAdded indicates a child's requested game was purchased.
	NotificationRequestedGameAdded NotificationType = 22
	// NotificationSendToPhone indicates a request to send info/link to phone.
	NotificationSendToPhone NotificationType = 23
	// NotificationClipDownloaded indicates a game clip was downloaded.
	NotificationClipDownloaded NotificationType = 24
	// Notification2FAPrompt indicates a 2FA authentication prompt.
	Notification2FAPrompt NotificationType = 25
	// NotificationMobileConfirmation indicates a new Steam Guard mobile confirmation.
	NotificationMobileConfirmation NotificationType = 26
	// NotificationPartnerEvent indicates a game developer event.
	NotificationPartnerEvent NotificationType = 27
	// NotificationPlaytestInvite indicates a playtest invitation.
	NotificationPlaytestInvite NotificationType = 28
	// NotificationTradeReversal indicates a trade reversal by Steam Support.
	NotificationTradeReversal NotificationType = 29
	// NotificationReportedContentAction indicates action was taken on reported content.
	NotificationReportedContentAction NotificationType = 30
)

// String returns the human-readable name of the notification type.
func (t NotificationType) String() string {
	switch t {
	case NotificationInvalid:
		return "Invalid"
	case NotificationTest:
		return "Test"
	case NotificationGift:
		return "Gift"
	case NotificationComment:
		return "Comment"
	case NotificationItem:
		return "Item"
	case NotificationFriendInvite:
		return "FriendInvite"
	case NotificationMajorSale:
		return "MajorSale"
	case NotificationPreloadAvailable:
		return "PreloadAvailable"
	case NotificationWishlist:
		return "Wishlist"
	case NotificationTradeOffer:
		return "TradeOffer"
	case NotificationGeneral:
		return "General"
	case NotificationHelpRequest:
		return "HelpRequest"
	case NotificationAsyncGame:
		return "AsyncGame"
	case NotificationChatMsg:
		return "ChatMsg"
	case NotificationModeratorMsg:
		return "ModeratorMsg"
	case NotificationParentalFeatureAccessRequest:
		return "ParentalFeatureAccessRequest"
	case NotificationFamilyInvite:
		return "FamilyInvite"
	case NotificationFamilyPurchaseRequest:
		return "FamilyPurchaseRequest"
	case NotificationParentalPlaytimeRequest:
		return "ParentalPlaytimeRequest"
	case NotificationFamilyPurchaseRequestResponse:
		return "FamilyPurchaseRequestResponse"
	case NotificationParentalFeatureAccessResponse:
		return "ParentalFeatureAccessResponse"
	case NotificationParentalPlaytimeResponse:
		return "ParentalPlaytimeResponse"
	case NotificationRequestedGameAdded:
		return "RequestedGameAdded"
	case NotificationSendToPhone:
		return "SendToPhone"
	case NotificationClipDownloaded:
		return "ClipDownloaded"
	case Notification2FAPrompt:
		return "2FAPrompt"
	case NotificationMobileConfirmation:
		return "MobileConfirmation"
	case NotificationPartnerEvent:
		return "PartnerEvent"
	case NotificationPlaytestInvite:
		return "PlaytestInvite"
	case NotificationTradeReversal:
		return "TradeReversal"
	case NotificationReportedContentAction:
		return "ReportedContentAction"
	default:
		return fmt.Sprintf("Unknown(%d)", uint32(t))
	}
}

// FromProtoNotificationType converts a protobuf ESteamNotificationType to NotificationType.
func FromProtoNotificationType(t pb.ESteamNotificationType) NotificationType {
	return NotificationType(t)
}

// ItemAnnouncementsEvent is emitted when Steam sends item announcement updates.
type ItemAnnouncementsEvent struct {
	bus.BaseEvent
	CountNewItems uint32
	UnseenItems   []*pb.CMsgClientItemAnnouncements_UnseenItem
}

// CommentNotificationsEvent is emitted when Steam sends comment notification updates.
type CommentNotificationsEvent struct {
	bus.BaseEvent
	CountNewComments              uint32
	CountNewCommentsOwner         uint32
	CountNewCommentsSubscriptions uint32
}

// UserNotificationsEvent is emitted when user notification counts change.
type UserNotificationsEvent struct {
	bus.BaseEvent
	Notifications map[NotificationType]uint32
}

// OfflineMessagesEvent is emitted when offline messages are received.
type OfflineMessagesEvent struct {
	bus.BaseEvent
	OfflineMessages            uint32
	FriendsWithOfflineMessages []id.ID
}

// MarketingMessagesEvent is emitted when marketing messages are received.
type MarketingMessagesEvent struct {
	bus.BaseEvent
	Timestamp int64
	Messages  []MarketingMessage
}

// MarketingMessage represents a single marketing message.
type MarketingMessage struct {
	ID    string
	URL   string
	Flags uint32
}

// ReceivedEvent is emitted when modern SteamNotificationClient events arrive.
type ReceivedEvent struct {
	bus.BaseEvent
	Notifications            []*pb.SteamNotificationData
	PendingGiftCount         uint32
	PendingFriendCount       uint32
	PendingFamilyInviteCount uint32
}
