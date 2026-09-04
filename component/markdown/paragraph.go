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

	"unstable.build/rune/component"
)

type paragraphBlock struct {
	content textRun
	cfg     *Config
	w       int // width from last Height call
}

var _ block = (*paragraphBlock)(nil)

func newParagraphBlock(content textRun, cfg *Config) *paragraphBlock {
	return &paragraphBlock{content: content, cfg: cfg}
}

func (p *paragraphBlock) Height(width int) int {
	if width <= 0 {
		return 0
	}
	lines := countWrappedLines(p.content, width)
	return lines + 1
}

func (p *paragraphBlock) Resize(width, _ int) {
	p.w = width
}

func (p *paragraphBlock) Draw(w term.Writer) {
	if p.w <= 0 {
		return
	}

	lines := wrapTextRun(p.content, p.w)

	if p.cfg.Paragraph.Bg != term.ColorDefault {
		bgAttr := term.Attributes{Bg: p.cfg.Paragraph.Bg}
		for y := range len(lines) {
			for x := range p.w {
				w.UnionAttributes(term.Coordinates{X: x, Y: y}, bgAttr)
			}
		}
	}
	for i, line := range lines {
		x := 0
		for _, sp := range line {
			attr := resolveStyle(sp.style, p.cfg)
			x = component.WriteText(w, x, i, p.w, sp.text, attr)
		}
	}
}

func (p *paragraphBlock) Dimensions() (width, height int) {
	return p.content.Width(), 2
}

func (p *paragraphBlock) SpanAt(x, y int) (text, url string, ok bool) {
	if p.w <= 0 {
		return
	}
	lines := wrapTextRun(p.content, p.w)
	if y < 0 || y >= len(lines) {
		return
	}
	return spanAtInLine(lines[y], x)
}

func (p *paragraphBlock) CharAt(x, y int) (rune, bool) {
	if p.w <= 0 {
		return 0, false
	}
	lines := wrapTextRun(p.content, p.w)
	if y < 0 || y >= len(lines) {
		return 0, false
	}
	return charAtInLine(lines[y], x)
}
