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

package vtescreen

import (
	"context"
)

type ctxKey int

var screenIDKey ctxKey

// NewContext returns a new Context that holds a screen context key,
// such that any contexts that derive from it, including itself, would
// return true in when passed on call to IsScreenContext.
func NewContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, screenIDKey, struct{}{})
}

// IsScreenContext returns the ID value stored in ctx, if any.
func IsScreenContext(ctx context.Context) bool {
	v := ctx.Value(screenIDKey)
	return v != nil
}
