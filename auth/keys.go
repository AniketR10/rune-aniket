// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package auth

import (
	"context"

	"github.com/unstablebuild/blue/auth"
)

// KeysCache returns a auth.Keys implementation that calls keys once,
// and caches the results of Verify and Sign forever.
func KeysCache(keys auth.Keys) auth.Keys {
	return &cache{root: keys}
}

type cache struct {
	root auth.Keys

	cachedSign   *auth.Key
	cachedVerify []auth.Key
}

func (c *cache) Sign(ctx context.Context) (auth.Key, error) {
	if c.cachedSign == nil {
		ret, err := c.root.Sign(ctx)
		if err == nil {
			c.cachedSign = new(auth.Key)
			*c.cachedSign = ret
		}
		return ret, err
	}
	return *c.cachedSign, nil
}

func (c *cache) Verify(ctx context.Context) ([]auth.Key, error) {
	if c.cachedVerify == nil {
		ret, err := c.root.Verify(ctx)
		if err == nil {
			c.cachedVerify = ret
		}
		return ret, err
	}
	return c.cachedVerify, nil
}
