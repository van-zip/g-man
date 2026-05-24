// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package friends

import (
	"context"
	"fmt"

	pb "github.com/lemon4ksan/g-man/pkg/protobuf/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/service"
)

// CreateFriendInviteToken creates a quick-invite token/link that allows any user to add you.
// limit: maximum number of uses allowed.
// duration: invite link duration in seconds.
func (m *Manager) CreateFriendInviteToken(ctx context.Context, limit, duration uint32) (string, error) {
	req := &pb.CUserAccount_CreateFriendInviteToken_Request{
		InviteLimit:    &limit,
		InviteDuration: &duration,
	}

	resp, err := service.WebAPI[pb.CUserAccount_CreateFriendInviteToken_Response](
		ctx,
		m.client,
		"POST",
		"UserAccount",
		"CreateFriendInviteToken",
		1,
		req,
	)
	if err != nil {
		return "", fmt.Errorf("friends: failed to create quick-invite token: %w", err)
	}

	return resp.GetInviteToken(), nil
}

// GetFriendInviteTokens retrieves the list of active quick-invite tokens for this account.
func (m *Manager) GetFriendInviteTokens(
	ctx context.Context,
) ([]*pb.CUserAccount_CreateFriendInviteToken_Response, error) {
	req := &pb.CUserAccount_GetFriendInviteTokens_Request{}

	resp, err := service.WebAPI[pb.CUserAccount_GetFriendInviteTokens_Response](
		ctx,
		m.client,
		"GET",
		"UserAccount",
		"GetFriendInviteTokens",
		1,
		req,
	)
	if err != nil {
		return nil, fmt.Errorf("friends: failed to list quick-invite tokens: %w", err)
	}

	return resp.GetTokens(), nil
}

// RevokeFriendInviteToken revokes (invalidates) an active quick-invite token.
func (m *Manager) RevokeFriendInviteToken(ctx context.Context, token string) error {
	req := &pb.CUserAccount_RevokeFriendInviteToken_Request{
		InviteToken: &token,
	}

	_, err := service.WebAPI[service.NoResponse](
		ctx,
		m.client,
		"POST",
		"UserAccount",
		"RevokeFriendInviteToken",
		1,
		req,
	)
	if err != nil {
		return fmt.Errorf("friends: failed to revoke quick-invite token: %w", err)
	}

	return nil
}

// ViewFriendInviteToken checks the validity of a quick-invite token belonging to another user.
func (m *Manager) ViewFriendInviteToken(
	ctx context.Context,
	steamID uint64,
	token string,
) (*pb.CUserAccount_ViewFriendInviteToken_Response, error) {
	req := &pb.CUserAccount_ViewFriendInviteToken_Request{
		Steamid:     &steamID,
		InviteToken: &token,
	}

	resp, err := service.WebAPI[pb.CUserAccount_ViewFriendInviteToken_Response](
		ctx,
		m.client,
		"GET",
		"UserAccount",
		"ViewFriendInviteToken",
		1,
		req,
	)
	if err != nil {
		return nil, fmt.Errorf("friends: failed to view quick-invite token: %w", err)
	}

	return resp, nil
}
