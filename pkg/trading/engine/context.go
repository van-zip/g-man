// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package engine

import (
	"context"
	"sync"

	"github.com/lemon4ksan/g-man/pkg/trading"
	"github.com/lemon4ksan/g-man/pkg/trading/reason"
)

// Verdict contains the final decision and the reasoning behind it.
type Verdict struct {
	Action trading.ActionType
	Reason reason.TradeReason
	Data   any // Optional payload (e.g., CounterOffer struct)
}

// Decision converts the engine verdict into a generic trading ActionDecision
// that the automated Processor understands.
func (v Verdict) Decision() trading.ActionDecision {
	d := trading.ActionDecision{
		Action: v.Action,
		Reason: v.Reason.String(),
	}

	if v.Action == trading.ActionCounter {
		if params, ok := v.Data.(*trading.CounterParams); ok {
			d.CounterParams = params
		}
	}

	// For the automated processor, Review/Ignore/Undecided are treated as Skip
	if d.Action == "" || d.Action == trading.ActionReview || d.Action == trading.ActionIgnore {
		d.Action = trading.ActionSkip
	}

	return d
}

// TradeContext flows through the middleware chain.
// It carries the original offer, execution context, and shared metadata.
type TradeContext struct {
	context.Context

	Offer   *trading.TradeOffer
	Verdict Verdict

	// Metadata storage
	mu   sync.RWMutex
	data map[string]any
}

// NewTradeContext creates a fresh context for an incoming offer.
func NewTradeContext(ctx context.Context, offer *trading.TradeOffer) *TradeContext {
	return &TradeContext{
		Context: ctx,
		Offer:   offer,
		Verdict: Verdict{Action: trading.ActionSkip}, // Default to skip (undecided)
		data:    make(map[string]any),
	}
}

// Set stores a key-value pair in the context's metadata. Safe for concurrent use.
func (c *TradeContext) Set(key string, val any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.data[key] = val
}

// Get retrieves a value from the context's metadata.
func (c *TradeContext) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	val, ok := c.data[key]

	return val, ok
}

// Accept sets the verdict to ACCEPT and stops further reasoning.
func (c *TradeContext) Accept(reason reason.TradeReason) {
	c.Verdict = Verdict{Action: trading.ActionAccept, Reason: reason}
}

// Decline sets the verdict to DECLINE and stops further reasoning.
func (c *TradeContext) Decline(reason reason.TradeReason) {
	c.Verdict = Verdict{Action: trading.ActionDecline, Reason: reason}
}

// Review marks the offer for manual review.
func (c *TradeContext) Review(reason reason.TradeReason) {
	c.Verdict = Verdict{Action: trading.ActionReview, Reason: reason}
}

// Counter sets the verdict to COUNTER and provides necessary parameters.
func (c *TradeContext) Counter(reason reason.TradeReason, params *trading.CounterParams) {
	c.Verdict = Verdict{Action: trading.ActionCounter, Reason: reason, Data: params}
}
