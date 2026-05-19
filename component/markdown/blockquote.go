// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.


package markdown

import (
	"github.com/unstablebuild/rune-go-sdk/term"
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
			w.SetCell(term.Coordinates{X: indent, Y: y + lineIdx}, term.Cell{
				Ch:         b.cfg.BlockquoteBorder,
				Width:      1,
				Attributes: b.cfg.Blockquote,
			})

			x := indent + blockquoteIndent
			for _, sp := range line {
				attr := resolveStyle(sp.style, b.cfg)
				attr = term.AttributesUnion(attr, b.cfg.Blockquote)
				for _, r := range sp.text {
					if x >= b.w {
						break
					}
					w.SetCell(term.Coordinates{X: x, Y: y + lineIdx}, term.Cell{
						Ch:         r,
						Width:      1,
						Attributes: attr,
					})
					x++
				}
			}
		}
		y += len(lines)
		if len(lines) == 0 {
			w.SetCell(term.Coordinates{X: indent, Y: y}, term.Cell{
				Ch:         b.cfg.BlockquoteBorder,
				Width:      1,
				Attributes: b.cfg.Blockquote,
			})
			y++
		}
	}

	if b.nested != nil {
		nestedHeight := b.nested.drawAtIndent(w, y, indent+blockquoteIndent)
		for i := 0; i < nestedHeight-1; i++ {
			w.SetCell(term.Coordinates{X: indent, Y: y + i}, term.Cell{
				Ch:         b.cfg.BlockquoteBorder,
				Width:      1,
				Attributes: b.cfg.Blockquote,
			})
		}
		y += nestedHeight
	}

	return y - startY + 1
}

func (b *blockquoteBlock) dimensionsAtIndent(indent int) (width, height int) {
	maxWidth := 0
	totalHeight := 0

	for _, content := range b.content {
		contentWidth := indent + blockquoteIndent + content.Len()
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
