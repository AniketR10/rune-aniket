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

package exoeditor

import (
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/text"
)

const maxMessageBarHeight = 20

type messageBar struct {
	text.Handler

	source    *editorHandler
	width     int
	height    int
	barHeight int
	lastMsg   string
}

func withMessageBar(inner text.Handler, source *editorHandler) text.Handler {
	return &messageBar{Handler: inner, source: source}
}

func (b *messageBar) Resize(width, height int) {
	b.width = width
	b.height = height
	b.lastMsg = b.source.LocationMessageAtCursor()
	b.barHeight = barHeightFor(b.lastMsg, width, height)
	b.Handler.Resize(width, height-b.barHeight)
}

func (b *messageBar) Draw(w term.Writer) {
	msg := b.source.LocationMessageAtCursor()
	want := barHeightFor(msg, b.width, b.height)
	if want != b.barHeight || msg != b.lastMsg {
		b.barHeight = want
		b.lastMsg = msg
		b.Handler.Resize(b.width, b.height-b.barHeight)
	}
	b.Handler.Draw(w)
	if b.barHeight == 0 {
		return
	}
	top := b.height - b.barHeight
	attrs := term.Attributes{Bg: term.ColorGray}
	x := 0
	y := top
	for _, r := range msg {
		if y >= b.height {
			break
		}
		w.SetCell(term.Coordinates{X: x, Y: y},
			term.NewCell(r, 0, attrs))
		x++
		if x >= b.width {
			x = 0
			y++
		}
	}
	for ; y < b.height; y++ {
		for ; x < b.width; x++ {
			w.SetCell(term.Coordinates{X: x, Y: y},
				term.NewCell(' ', 0, attrs))
		}
		x = 0
	}
	offset := term.Attributes{Attrs: term.AttrVerticalRenderOffset}
	for y := top; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			w.UnionAttributes(term.Coordinates{X: x, Y: y}, offset)
		}
	}
}

func barHeightFor(msg string, width, height int) int {
	if msg == "" || width <= 0 || height <= 0 {
		return 0
	}
	runes := 0
	for range msg {
		runes++
	}
	rows := (runes + width - 1) / width
	rows = min(rows, maxMessageBarHeight, height)
	return rows
}
