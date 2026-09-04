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

package handler

import (
	compapi "github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/rune/cell"
	"unstable.build/rune/component"
)

// FloatingBuffer wraps a tui.Handler and uses the contents of buffer
// do determine the best dimensions for the given handler.
func FloatingBuffer(h tui.Handler, buffer *cell.Buffer) handler.Floating {
	return floatingBuffer{h: h, Floating: component.FloatingBuffer(h, buffer)}
}

type floatingBuffer struct {
	h tui.Handler
	compapi.Floating
}

func (f floatingBuffer) Handle(ev term.Event) (exit, handled bool) {
	return f.h.Handle(ev)
}

func (f floatingBuffer) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return f.h.Cursor()
}

func (f floatingBuffer) Selection() (string, bool) {
	return f.h.Selection()
}
