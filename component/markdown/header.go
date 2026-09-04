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

type headerBlock struct {
	level   int
	content textRun
	cfg     *Config
	w       int // width from last Height call
}

var _ block = (*headerBlock)(nil)

func newHeaderBlock(level int, content textRun, cfg *Config) *headerBlock {
	return &headerBlock{level: level, content: content, cfg: cfg}
}

func (h *headerBlock) Height(width int) int {
	if width <= 0 {
		return 0
	}
	prefixLen := h.prefixLen()
	effectiveWidth := max(1, width-prefixLen)
	lines := countWrappedLines(h.content, effectiveWidth)
	// 1 (above) + content lines + 1 (standard spacing)
	return lines + 2
}

func (h *headerBlock) Resize(width, _ int) {
	h.w = width
}

func (hb *headerBlock) Draw(w term.Writer) {
	if hb.w <= 0 {
		return
	}

	attr := hb.getAttr()
	prefixLen := hb.prefixLen()
	effectiveWidth := max(1, hb.w-prefixLen)
	lines := wrapTextRun(hb.content, effectiveWidth)
	startY := 1

	if attr.Bg != term.ColorDefault {
		maxContentWidth := 0
		for i, line := range lines {
			lineWidth := line.Width()
			if i == 0 {
				lineWidth += prefixLen
			} else {
				lineWidth += prefixLen
			}
			if lineWidth > maxContentWidth {
				maxContentWidth = lineWidth
			}
		}

		bgWidth := min(maxContentWidth+2, hb.w)
		bgAttr := term.Attributes{Bg: attr.Bg}
		for lineIdx := range lines {
			y := startY + lineIdx
			for x := range bgWidth {
				w.UnionAttributes(term.Coordinates{X: x, Y: y}, bgAttr)
			}
		}
	}

	contentOffset := 0
	if attr.Bg != term.ColorDefault {
		contentOffset = 1
	}

	for i, line := range lines {
		y := startY + i
		x := contentOffset

		if i == 0 && hb.cfg.HeaderPrefix {
			for j := 0; j < hb.level; j++ {
				if x >= hb.w {
					break
				}
				w.SetCell(term.Coordinates{X: x, Y: y}, term.NewCell('#', 1, attr))
				x++
			}
			if x < hb.w {
				w.SetCell(term.Coordinates{X: x, Y: y}, term.NewCell(' ', 1, attr))
				x++
			}
		} else if i > 0 {
			x = contentOffset + prefixLen
		}

		for _, sp := range line {
			x = component.WriteText(w, x, y, hb.w, sp.text, attr)
		}
	}
}

func (h *headerBlock) Dimensions() (width, height int) {
	prefixLen := h.prefixLen()
	// 1 (above) + 1 (content line) + 1 (standard spacing)
	return prefixLen + h.content.Width(), 3
}

func (hb *headerBlock) SpanAt(x, y int) (text, url string, ok bool) {
	if hb.w <= 0 {
		return
	}
	if y < 1 {
		return
	}
	y--

	prefixLen := hb.prefixLen()
	effectiveWidth := max(1, hb.w-prefixLen)
	lines := wrapTextRun(hb.content, effectiveWidth)
	if y >= len(lines) {
		return
	}

	attr := hb.getAttr()
	contentOffset := 0
	if attr.Bg != 0 {
		contentOffset = 1
	}

	if y == 0 {
		x -= contentOffset + prefixLen
	} else {
		x -= contentOffset + prefixLen
	}

	if x < 0 {
		return
	}
	return spanAtInLine(lines[y], x)
}

func (hb *headerBlock) CharAt(x, y int) (rune, bool) {
	if hb.w <= 0 {
		return 0, false
	}
	if y < 1 {
		return 0, false
	}
	y--

	prefixLen := hb.prefixLen()
	effectiveWidth := max(1, hb.w-prefixLen)
	lines := wrapTextRun(hb.content, effectiveWidth)
	if y >= len(lines) {
		return 0, false
	}

	attr := hb.getAttr()
	contentOffset := 0
	if attr.Bg != 0 {
		contentOffset = 1
	}

	if y == 0 && x >= contentOffset && x < contentOffset+prefixLen {
		prefixIdx := x - contentOffset
		if prefixIdx < hb.level {
			return '#', true
		}
		return ' ', true
	}

	x -= contentOffset + prefixLen
	if x < 0 {
		return 0, false
	}
	return charAtInLine(lines[y], x)
}

func (h *headerBlock) prefixLen() int {
	if !h.cfg.HeaderPrefix {
		return 0
	}
	return h.level + 1 // "# ", "## ", etc.
}

func (h *headerBlock) getAttr() term.Attributes {
	switch h.level {
	case 1:
		return h.cfg.H1
	case 2:
		return h.cfg.H2
	case 3:
		return h.cfg.H3
	case 4:
		return h.cfg.H4
	case 5:
		return h.cfg.H5
	default:
		return h.cfg.H6
	}
}
