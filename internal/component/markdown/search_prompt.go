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

package markdown

import (
	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/rune/internal/component"
)

type searchResult int

const (
	searchIgnored searchResult = iota
	searchConsumed
	searchConfirm
	searchCancel
)

// searchPrompt manages a less-style "/" search input bar.
type searchPrompt struct {
	active bool
	buf    []rune
}

func (p *searchPrompt) isActive() bool { return p.active }

func (p *searchPrompt) open() { p.active = true; p.buf = p.buf[:0] }

func (p *searchPrompt) query() string { return string(p.buf) }

func (p *searchPrompt) handleKey(ev term.Event) searchResult {
	if !p.active {
		return searchIgnored
	}

	switch ev.Key {
	case term.KeyEsc:
		p.active = false
		return searchCancel
	case term.KeyEnter:
		p.active = false
		return searchConfirm
	case term.KeyBackspace:
		if len(p.buf) > 0 {
			p.buf = p.buf[:len(p.buf)-1]
		}
		return searchConsumed
	}

	if ev.Ch != 0 {
		p.buf = append(p.buf, ev.Ch)
		return searchConsumed
	}

	return searchConsumed
}

func (p *searchPrompt) draw(w term.Writer, y, width int) {
	if !p.active || width <= 0 {
		return
	}

	for x := range width {
		w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
			Ch: ' ', Width: 1,
		})
	}

	w.SetCell(term.Coordinates{X: 0, Y: y}, term.NewCell('/', 1, term.Attributes{Attrs: term.AttrBold}))

	component.WriteText(w, 1, y, width, string(p.buf), term.Attributes{})
}

func (p *searchPrompt) cursor(y int) (term.Coordinates, term.CursorStyle, bool) {
	if !p.active {
		return term.Coordinates{}, term.CursorStyleDefault, false
	}
	return term.Coordinates{X: textWidth(string(p.buf)) + 1, Y: y},
		term.CursorStyleDefault, true
}
