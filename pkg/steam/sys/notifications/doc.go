// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package notifications handles incoming Steam notification messages
// and routes them to the event bus.
//
// It listens for various notification types including item announcements,
// comment notifications, user notifications, offline messages, marketing
// messages, and modern SteamNotificationClient events.
//
// # Key Components
//
//   - [Notifications]: The central module manager that handles incoming notification packets.
//   - [ItemAnnouncementsEvent]: Emitted when new item announcements are received.
//   - [CommentNotificationsEvent]: Emitted when new comment notifications are received.
//   - [UserNotificationsEvent]: Emitted when user notification counts change.
//   - [OfflineMessagesEvent]: Emitted when offline messages are received.
//   - [MarketingMessagesEvent]: Emitted when marketing messages are received.
//   - [NotificationsReceivedEvent]: Emitted when modern SteamNotificationClient events arrive.
//
// # Basic Usage Example
//
//	package main
//
//	import (
//		"fmt"
//		"github.com/lemon4ksan/g-man/pkg/steam"
//		"github.com/lemon4ksan/g-man/pkg/steam/sys/notifications"
//	)
//
//	func main() {
//		client, _ := steam.NewClient(steam.DefaultConfig(),
//			notifications.WithModule(),
//		)
//		defer client.Close()
//
//		n := notifications.From(client)
//		_ = n // use the module
//	}
package notifications
