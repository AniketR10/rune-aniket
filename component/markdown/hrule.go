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
)

type horizontalRuleBlock struct {
	cfg *Config
	w   int // width from last Height call
}

var _ block = (*horizontalRuleBlock)(nil)

func newHorizontalRuleBlock(cfg *Config) *horizontalRuleBlock {
	return &horizontalRuleBlock{cfg: cfg}
}

func (hr *horizontalRuleBlock) Height(width int) int {
	if width <= 0 {
		return 0
	}
	return 2
}

func (hr *horizontalRuleBlock) Resize(width, _ int) {
	hr.w = width
}

func (hr *horizontalRuleBlock) Draw(w term.Writer) {
	if hr.w <= 0 {
		return
	}

	ch := hr.cfg.HorizontalRule
	attr := hr.cfg.HorizontalRuleAttr

	if attr.Bg != term.ColorDefault {
		bgAttr := term.Attributes{Bg: attr.Bg}
		for x := range hr.w {
			w.UnionAttributes(term.Coordinates{X: x, Y: 0}, bgAttr)
		}
	}

	for x := range hr.w {
		w.SetCell(term.Coordinates{X: x, Y: 0}, term.NewCell(ch, 1, attr))
	}
}

func (hr *horizontalRuleBlock) Dimensions() (width, height int) {
	return 1, 2
}

func (hr *horizontalRuleBlock) SpanAt(
	x, y int,
) (text, url string, ok bool) {
	return
}

func (hr *horizontalRuleBlock) CharAt(x, y int) (rune, bool) {
	if y == 0 && x >= 0 && x < hr.w {
		return hr.cfg.HorizontalRule, true
	}
	return 0, false
}
