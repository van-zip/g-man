// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package trading

import (
	"github.com/lemon4ksan/g-man/pkg/tf2/schema"
	"github.com/lemon4ksan/g-man/pkg/trading"
)

// IsJunk returns true if the item is considered "junk" in TF2.
// Junk items include crates/cases (unless they are rare/special)
// and items without a valid SKU.
func IsJunk(it *trading.Item) bool {
	if it.SKU == "" {
		return true
	}

	// Check for crates/cases
	for _, attr := range it.Attributes {
		if attr.Defindex == schema.AttrCrateSeries {
			return true
		}
	}

	return false
}
