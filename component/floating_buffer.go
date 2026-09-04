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

package component

import (
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/rune/cell"
)

// FloatingBuffer wraps a tui.Component and uses the contents of buffer
// do determine the best dimensions for the given component.
func FloatingBuffer(c tui.Component, buffer *cell.Buffer) component.Floating {
	return floatingBuffer{Component: c, buffer: buffer}
}

type floatingBuffer struct {
	tui.Component
	buffer *cell.Buffer
}

func (f floatingBuffer) Dimensions() (width, height int) {
	width = term.CalculateOptimalWidth(f.buffer.RawCells())
	height = f.buffer.Rows()
	return
}
