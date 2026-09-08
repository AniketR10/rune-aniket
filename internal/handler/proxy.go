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

var _ tui.Handler = (*Proxy)(nil)

// Proxy satisfies tui.Handler by taking a pointer to a tui.Handler
// and dereferencing on each method call. Caller is responsible for
// initializing Ptr correctly.
type Proxy struct {
	Target tui.Handler
}

// Resize satisfies tui.Handler.
func (i *Proxy) Resize(width, height int) {
	i.Target.Resize(width, height)
}

// Draw satisfies tui.Handler.
func (i *Proxy) Draw(w term.Writer) {
	i.Target.Draw(w)
}

// Handle satisfies tui.Handler.
func (i *Proxy) Handle(ev term.Event) (exit, handled bool) {
	return i.Target.Handle(ev)
}

// Cursor satisfies tui.Handler.
func (i *Proxy) Cursor() (pos term.Coordinates, style term.CursorStyle, show bool) {
	return i.Target.Cursor()
}

// Selection satisfies tui.Handler.
func (i *Proxy) Selection() (string, bool) {
	return i.Target.Selection()
}
