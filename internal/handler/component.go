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

type withComponent struct {
	tui.Handler
	c tui.Component
}

// WithComponent wraps h with comp for the methods of h that satisfy tui.Component
// and delegates the remaining tui.Handler methods to h.
func WithComponent(h tui.Handler, comp tui.Component) tui.Handler {
	return withComponent{Handler: h, c: comp}
}

func (c withComponent) Draw(w term.Writer) {
	c.c.Draw(w)
}

func (c withComponent) Resize(width, height int) {
	c.c.Resize(width, height)
}
