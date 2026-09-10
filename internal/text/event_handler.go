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

package text

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
)

// EventHandler wraps the basic method Handle.
type EventHandler interface {
	// Handle handles Event and returns true if it no longer needs to receive events,
	// in other words it returns true if it's done processing events.
	Handle(context.Context, textapi.Event) bool
}

type fnEventHandler struct {
	cb func(context.Context, textapi.Event) bool
}

func (f *fnEventHandler) Handle(ctx context.Context, ev textapi.Event) bool {
	return f.cb(ctx, ev)
}

// FuncEventHandler returns an EventHandler that calls fn
// every time Handle is invoked.
func FuncEventHandler(fn func(context.Context, textapi.Event) bool) EventHandler {
	return &fnEventHandler{
		cb: fn,
	}
}
