// Copyright (c) 2026 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package limits queries WhatsApp's account-level messaging limits (new-chat
// message capping and reachout timelock) — the limits behind error 463 on sends
// to new contacts.
//
// It is a port of the fork's former vendored whatsmeow (whatsmeow-lib/limits.go)
// on top of the official go.mau.fi/whatsmeow, using the public
// Client.DangerousInternals().SendMexIQ instead of patching the library.
package limits

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"go.mau.fi/whatsmeow"
)

const (
	QueryNewChatMessageCappingInfo = "24503548349331633"
	QueryAccountReachoutTimelock   = "23983697327930364"
)

// MexSender is the subset of the whatsmeow internals used here (satisfied by
// (*whatsmeow.Client).DangerousInternals()). It exists so the parsing can be tested.
type MexSender interface {
	SendMexIQ(ctx context.Context, queryID string, variables any) (json.RawMessage, error)
}

var errNilClient = errors.New("whatsmeow client is nil")

type respGetNewChatMessageCappingInfo struct {
	MessageCappingInfo *NewChatMessageCappingInfo `json:"xwa2_message_capping_info"`
}

type respGetAccountReachoutTimelock struct {
	ReachoutTimelock *AccountReachoutTimelock `json:"xwa2_fetch_account_reachout_timelock"`
}

// GetNewChatMessageCappingInfo fetches raw MEX capping info for caller-invoked
// new-chat messaging (the quota of how many brand-new chats the account may start
// in the current cycle). When the quota is exhausted, sends to new contacts get error 463.
func GetNewChatMessageCappingInfo(ctx context.Context, cli *whatsmeow.Client) (*NewChatMessageCappingInfo, error) {
	if cli == nil {
		return nil, errNilClient
	}
	return FetchNewChatMessageCappingInfo(ctx, cli.DangerousInternals())
}

// GetAccountReachoutTimelock fetches raw MEX reachout timelock info (whether the
// account is currently timelocked from reaching out to new contacts, and until when).
func GetAccountReachoutTimelock(ctx context.Context, cli *whatsmeow.Client) (*AccountReachoutTimelock, error) {
	if cli == nil {
		return nil, errNilClient
	}
	return FetchAccountReachoutTimelock(ctx, cli.DangerousInternals())
}

// FetchNewChatMessageCappingInfo is GetNewChatMessageCappingInfo over any MexSender.
func FetchNewChatMessageCappingInfo(ctx context.Context, mex MexSender) (*NewChatMessageCappingInfo, error) {
	data, err := mex.SendMexIQ(ctx, QueryNewChatMessageCappingInfo, map[string]any{
		"input": map[string]any{
			"type": "INDIVIDUAL_NEW_CHAT_MSG",
		},
	})
	var respData respGetNewChatMessageCappingInfo
	if data != nil {
		jsonErr := json.Unmarshal(data, &respData)
		if err == nil && jsonErr != nil {
			err = jsonErr
		} else if err == nil && respData.MessageCappingInfo == nil {
			err = fmt.Errorf("mex unexpected null response for new chat message capping info")
		}
	}
	return respData.MessageCappingInfo, err
}

// FetchAccountReachoutTimelock is GetAccountReachoutTimelock over any MexSender.
func FetchAccountReachoutTimelock(ctx context.Context, mex MexSender) (*AccountReachoutTimelock, error) {
	data, err := mex.SendMexIQ(ctx, QueryAccountReachoutTimelock, map[string]any{})
	var respData respGetAccountReachoutTimelock
	if data != nil {
		jsonErr := json.Unmarshal(data, &respData)
		if err == nil && jsonErr != nil {
			err = jsonErr
		} else if err == nil && respData.ReachoutTimelock == nil {
			err = fmt.Errorf("mex unexpected null response for fetching reachout timelock")
		}
	}
	return respData.ReachoutTimelock, err
}
