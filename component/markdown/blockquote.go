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

const blockquoteIndent = 2

type blockquoteBlock struct {
	content []textRun
	nested  *blockquoteBlock
	cfg     *Config
	w       int // width from last Height call
}

var _ block = (*blockquoteBlock)(nil)

func newBlockquoteBlock(
	content []textRun, nested *blockquoteBlock, cfg *Config,
) *blockquoteBlock {
	return &blockquoteBlock{
		content: content,
		nested:  nested,
		cfg:     cfg,
	}
}

func (b *blockquoteBlock) Height(width int) int {
	return b.heightAtIndent(width, 0)
}

func (b *blockquoteBlock) Resize(width, _ int) {
	b.w = width
	if b.nested != nil {
		b.nested.Resize(width, 0)
	}
}

func (b *blockquoteBlock) Draw(w term.Writer) {
	contentHeight := b.heightAtIndent(b.w, 0)
	if b.cfg.Blockquote.Bg != term.ColorDefault && contentHeight > 1 {
		bgAttr := term.Attributes{Bg: b.cfg.Blockquote.Bg}
		for y := range contentHeight - 1 {
			for x := range b.w {
				w.UnionAttributes(term.Coordinates{X: x, Y: y}, bgAttr)
			}
		}
	}
	b.drawAtIndent(w, 0, 0)
}

func (b *blockquoteBlock) Dimensions() (width, height int) {
	return b.dimensionsAtIndent(0)
}

func (b *blockquoteBlock) SpanAt(x, y int) (text, url string, ok bool) {
	return b.spanAtIndent(x, y, 0)
}

func (b *blockquoteBlock) CharAt(x, y int) (rune, bool) {
	return b.charAtIndent(x, y, 0)
}

func (b *blockquoteBlock) heightAtIndent(width, indent int) int {
	if width <= indent+blockquoteIndent {
		return 0
	}

	h := 0
	effectiveWidth := width - indent - blockquoteIndent

	for _, content := range b.content {
		lines := countWrappedLines(content, effectiveWidth)
		h += lines
	}

	if b.nested != nil {
		h += b.nested.heightAtIndent(width, indent+blockquoteIndent)
	}

	return h + 1
}

func (b *blockquoteBlock) drawAtIndent(w term.Writer, y, indent int) int {
	if b.w <= indent+blockquoteIndent {
		return 0
	}

	startY := y
	effectiveWidth := b.w - indent - blockquoteIndent

	for _, content := range b.content {
		lines := wrapTextRun(content, effectiveWidth)
		for lineIdx, line := range lines {
			w.SetCell(term.Coordinates{X: indent, Y: y + lineIdx}, term.NewCell(b.cfg.BlockquoteBorder, 1, b.cfg.Blockquote))

			x := indent + blockquoteIndent
			for _, sp := range line {
				attr := resolveStyle(sp.style, b.cfg)
				attr = term.AttributesUnion(attr, b.cfg.Blockquote)
				x = component.WriteText(w, x, y+lineIdx, b.w, sp.text, attr)
			}
		}
		y += len(lines)
		if len(lines) == 0 {
			w.SetCell(term.Coordinates{X: indent, Y: y}, term.NewCell(b.cfg.BlockquoteBorder, 1, b.cfg.Blockquote))
			y++
		}
	}

	if b.nested != nil {
		nestedHeight := b.nested.drawAtIndent(w, y, indent+blockquoteIndent)
		for i := 0; i < nestedHeight-1; i++ {
			w.SetCell(term.Coordinates{X: indent, Y: y + i}, term.NewCell(b.cfg.BlockquoteBorder, 1, b.cfg.Blockquote))
		}
		y += nestedHeight
	}

	return y - startY + 1
}

func (b *blockquoteBlock) dimensionsAtIndent(indent int) (width, height int) {
	maxWidth := 0
	totalHeight := 0

	for _, content := range b.content {
		contentWidth := indent + blockquoteIndent + content.Width()
		if contentWidth > maxWidth {
			maxWidth = contentWidth
		}
		totalHeight++
	}

	if b.nested != nil {
		nestedW, nestedH := b.nested.dimensionsAtIndent(indent + blockquoteIndent)
		if nestedW > maxWidth {
			maxWidth = nestedW
		}
		totalHeight += nestedH - 1
	}

	return maxWidth, totalHeight + 1
}

func (b *blockquoteBlock) spanAtIndent(x, y, indent int) (text, url string, ok bool) {
	if b.w <= indent+blockquoteIndent {
		return
	}

	effectiveWidth := b.w - indent - blockquoteIndent
	currentY := 0

	for _, content := range b.content {
		lines := wrapTextRun(content, effectiveWidth)
		lineCount := len(lines)
		if lineCount == 0 {
			lineCount = 1
		}

		if y >= currentY && y < currentY+lineCount {
			lineIdx := y - currentY
			if lineIdx < len(lines) {
				adjustedX := x - indent - blockquoteIndent
				if adjustedX >= 0 {
					return spanAtInLine(lines[lineIdx], adjustedX)
				}
			}
			return
		}
		currentY += lineCount
	}

	if b.nested != nil {
		nestedHeight := b.nested.heightAtIndent(b.w, indent+blockquoteIndent) - 1
		if y >= currentY && y < currentY+nestedHeight {
			return b.nested.spanAtIndent(x, y-currentY, indent+blockquoteIndent)
		}
	}
	return
}

func (b *blockquoteBlock) charAtIndent(x, y, indent int) (rune, bool) {
	if b.w <= indent+blockquoteIndent {
		return 0, false
	}

	effectiveWidth := b.w - indent - blockquoteIndent
	currentY := 0

	for _, content := range b.content {
		lines := wrapTextRun(content, effectiveWidth)
		lineCount := len(lines)
		if lineCount == 0 {
			lineCount = 1
		}

		if y >= currentY && y < currentY+lineCount {
			lineIdx := y - currentY
			if x == indent {
				return b.cfg.BlockquoteBorder, true
			}
			if lineIdx < len(lines) {
				adjustedX := x - indent - blockquoteIndent
				if adjustedX >= 0 {
					return charAtInLine(lines[lineIdx], adjustedX)
				}
			}
			return 0, false
		}
		currentY += lineCount
	}

	if b.nested != nil {
		nestedHeight := b.nested.heightAtIndent(b.w, indent+blockquoteIndent) - 1
		if y >= currentY && y < currentY+nestedHeight {
			return b.nested.charAtIndent(x, y-currentY, indent+blockquoteIndent)
		}
	}
	return 0, false
}
