// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/lemon4ksan/g-man/pkg/log"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/id"
	"github.com/lemon4ksan/g-man/pkg/steam/protocol/enums"
	"github.com/lemon4ksan/g-man/pkg/steam/social/chat"
	"github.com/lemon4ksan/g-man/pkg/steam/social/chat/commands"
	"github.com/lemon4ksan/g-man/pkg/steam/social/friends"
	webtrading "github.com/lemon4ksan/g-man/pkg/trading/web"
)

// ChatBot links Steam chat events with the command engine
type ChatBot struct {
	client     *steam.Client
	cmdManager *commands.Manager
	logger     log.Logger
}

func NewChatBot(client *steam.Client, logger log.Logger) *ChatBot {
	cmdManager := commands.From(client)

	return &ChatBot{
		client:     client,
		cmdManager: cmdManager,
		logger:     logger.With(log.Module("admin_bot")),
	}
}

// RegisterCommands declares the syntax and validation rules for chat commands
func (bot *ChatBot) RegisterCommands() {
	// 1. Simple informational command
	bot.cmdManager.Register("status", bot.handleStatus,
		commands.WithDescription("Shows the current status and uptime of the bot"),
	)

	// 2. Command with typed arguments and validation (Withdrawal)
	bot.cmdManager.Register("withdraw", bot.handleWithdraw,
		commands.WithDescription("Requests a transfer of a specific amount of funds to the specified SteamID"),
		commands.WithArgsSchema(
			commands.Required[id.ID]("target_id"),
			commands.Required[float64]("amount"),
		),
		commands.WithAdmin(), // Command is available to administrators only
	)

	// 3. Command for manual trade confirmation
	bot.cmdManager.Register("approve", bot.handleApprove,
		commands.WithDescription("Forcibly confirms an incoming trade by its ID"),
		commands.WithArgsSchema(
			commands.Required[uint64]("offer_id"),
		),
		commands.WithAdmin(),
	)
}

// ListenEvents subscribes to the event bus to handle incoming requests
func (bot *ChatBot) ListenEvents(ctx context.Context) {
	// Subscribe to relationship changes (friend requests) and messages
	sub := bot.client.Bus().Subscribe(
		&friends.RelationshipChangedEvent{},
	)

	go func() {
		defer sub.Unsubscribe()

		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-sub.C():
				if !ok {
					return
				}

				if e, ok := ev.(*friends.RelationshipChangedEvent); ok {
					bot.handleFriendRequest(ctx, e)
				}
			}
		}
	}()
}

// handleFriendRequest automatically approves incoming friend requests
func (bot *ChatBot) handleFriendRequest(ctx context.Context, e *friends.RelationshipChangedEvent) {
	friendsMgr := friends.From(bot.client)
	if e.New == enums.EFriendRelationship_RequestInitiator {
		bot.logger.Info("Received incoming friend request", log.String("steam_id", e.SteamID.String()))

		// Approve request via Web API method of the friends module
		err := friendsMgr.AcceptFriendRequestWeb(ctx, e.SteamID)
		if err != nil {
			bot.logger.Error("Failed to accept friend request", log.Err(err))
			return
		}

		// Send a welcome message
		chatMgr := chat.From(bot.client)
		_ = chatMgr.SendMessage(
			ctx,
			e.SteamID.Uint64(),
			"Hello! I am a trading bot. Type !help for a list of commands.",
		)
	}
}

// Handler for the !status command
func (bot *ChatBot) handleStatus(ctx context.Context, senderID uint64, args []string) (string, error) {
	return "🤖 The bot is operating normally. All services are active.", nil
}

// Handler for the !withdraw command with type validation (SteamID and float64)
func (bot *ChatBot) handleWithdraw(ctx context.Context, senderID uint64, args []any) (string, error) {
	targetID := args[0].(id.ID)
	amount := args[1].(float64)

	if amount <= 0 {
		return "", errors.New("withdrawal amount must be a positive number")
	}

	bot.logger.Warn("Withdraw executed by admin",
		log.Uint64("admin_id", senderID),
		log.String("target_id", targetID.String()),
		log.Float64("amount", amount),
	)

	return fmt.Sprintf(
		"✅ Withdrawal transaction of %0.2f has been successfully queued for %s",
		amount,
		targetID.String(),
	), nil
}

// Handler for the !approve command to approve trades by ID via the web trading module
func (bot *ChatBot) handleApprove(ctx context.Context, senderID uint64, args []any) (string, error) {
	offerID := args[0].(uint64)

	webTradeMgr := webtrading.From(bot.client)

	// Accept the offer directly via the web manager
	err := webTradeMgr.AcceptOffer(ctx, offerID)
	if err != nil {
		return "", fmt.Errorf("failed to approve trade #%d: %w", offerID, err)
	}

	bot.logger.Info("Admin approved trade manual", log.Uint64("admin", senderID), log.Uint64("offer", offerID))

	return fmt.Sprintf("✅ Trade #%d has been successfully confirmed and sent for verification.", offerID), nil
}
