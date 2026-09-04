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
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/cell"
)

// Buffer wraps a cell.Buffer and returns a tui.Component which satisfies
// Responsive. Note that this is not the most efficient implementation of tui.Component
// for a cell.Buffer. See component.Scroll for more details.
func Buffer(
	buf *cell.Buffer, cfg component.StringResponsiveConfig,
) component.Responsive {
	ret := &respBuf{
		buf: buf,
		cfg: cfg,
	}
	ret.ResponsiveString.Init(buf.RawCells(), cfg)
	return ret
}

var _ component.WithAttributes = (*respBuf)(nil)
var _ component.Scrollable = (*respBuf)(nil)
var _ component.Responsive = (*respBuf)(nil)
var _ fmt.Stringer = (*respBuf)(nil)

type respBuf struct {
	component.ResponsiveString
	cfg           component.StringResponsiveConfig
	buf           *cell.Buffer
	width, height int
}

func (b *respBuf) Height(width int) int {
	b.ResponsiveString.Reset(b.buf.RawCells(), b.cfg)
	return b.ResponsiveString.Height(width)
}

func (b *respBuf) Resize(width, height int) {
	b.width, b.height = width, height
	b.ResponsiveString.Resize(width, height)
}

func (b *respBuf) Draw(w term.Writer) {
	b.ResponsiveString.Reset(b.buf.RawCells(), b.cfg)
	b.ResponsiveString.Resize(b.width, b.height)
	b.ResponsiveString.Draw(w)
}

func (b *respBuf) MaxSeekOffset() int {
	return 0
}

func (b *respBuf) SeekDown() bool {
	return false
}

func (b *respBuf) SeekUp() bool {
	return false
}

func (b *respBuf) SeekOffset() int {
	return 0
}
