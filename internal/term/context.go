// Copyright (C) 2017-2026 The Rune Authors
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

package term

import (
	"context"
)

// key is an unexported type for keys defined in this package.
// This prevents collisions with keys defined in other packages.
type ctxKey int

// pKey is the key for sync.Payload values in Contexts. It is
// unexported; clients use workspace.ContextWithPayload and
// PayloadFromContext instead of using this key directly.
var pKey ctxKey

// ContextWithPayload returns a new Context that holds locker.
//
// Deprecated: use term.Event.Context to pass a context.
func ContextWithPayload(ctx context.Context, payload []byte) context.Context {
	return context.WithValue(ctx, pKey, payload)
}

// PayloadFromContext returns the payload value stored in ctx, if any.
//
// Deprecated: use term.Event.Context to pass a context.
func PayloadFromContext(ctx context.Context) ([]byte, bool) {
	locker, ok := ctx.Value(pKey).([]byte)
	return locker, ok
}
