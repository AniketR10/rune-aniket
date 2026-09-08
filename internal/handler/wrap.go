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
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

// Wrap wraps a tui.Handler with a fn that gets called
// instead of Handle called on h. The rest of tui.Handler
// methods are delegated directly to h, so if h.Handle needs to be
// called, it's the client's responsibility to do so.
//
// Additionally to the usual term.Events, term.EventResize events
// are also dispatched as events to fn, after Resize has been called on h.
func Wrap(h tui.Handler, fn func(term.Event) (bool, bool)) tui.Handler {
	return wrapHandler{h: h, fn: fn}
}

type wrapHandler struct {
	h  tui.Handler
	fn func(term.Event) (bool, bool)
}

func (n wrapHandler) Resize(width, height int) {
	n.h.Resize(width, height)
	n.fn(term.Event{Type: term.EventResize, Width: width, Height: height})
}

func (n wrapHandler) Draw(w term.Writer) {
	n.h.Draw(w)
}

func (n wrapHandler) Handle(ev term.Event) (exit, handled bool) {
	return n.fn(ev)
}

func (n wrapHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return n.h.Cursor()
}

func (n wrapHandler) Selection() (string, bool) {
	return n.h.Selection()
}
