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
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/term"
)

const listIndent = 2

type listItem struct {
	content textRun
	isTask  bool
	checked bool
	nested  *listBlock
}

type listBlock struct {
	ordered bool
	start   int
	items   []listItem
	cfg     *Config
	w       int // width from last Height call
}

var _ block = (*listBlock)(nil)

func newListBlock(
	ordered bool, start int, items []listItem, cfg *Config,
) *listBlock {
	return &listBlock{
		ordered: ordered,
		start:   start,
		items:   items,
		cfg:     cfg,
	}
}

func (l *listBlock) Height(width int) int {
	return l.heightAtIndent(width, 0)
}

func (l *listBlock) Resize(width, _ int) {
	l.w = width
	for _, item := range l.items {
		if item.nested != nil {
			item.nested.Resize(width, 0)
		}
	}
}

func (l *listBlock) Draw(w term.Writer) {
	contentHeight := l.heightAtIndent(l.w, 0)
	if l.cfg.Paragraph.Bg != term.ColorDefault && contentHeight > 1 {
		bgAttr := term.Attributes{Bg: l.cfg.Paragraph.Bg}
		for y := range contentHeight - 1 {
			for x := range l.w {
				w.UnionAttributes(term.Coordinates{X: x, Y: y}, bgAttr)
			}
		}
	}
	l.drawAtIndent(w, 0, 0)
}

func (l *listBlock) Dimensions() (width, height int) {
	return l.dimensionsAtIndent(0)
}

func (l *listBlock) SpanAt(x, y int) (text, url string, ok bool) {
	return l.spanAtIndent(x, y, 0)
}

func (l *listBlock) CharAt(x, y int) (rune, bool) {
	return l.charAtIndent(x, y, 0)
}

func (l *listBlock) heightAtIndent(width, indent int) int {
	if width <= indent+listIndent {
		return 0
	}

	h := 0
	effectiveWidth := width - indent - listIndent

	for _, item := range l.items {
		lines := countWrappedLines(item.content, effectiveWidth)
		h += lines

		if item.nested != nil {
			h += item.nested.heightAtIndent(width, indent+listIndent)
		}
	}

	// Only add spacing at top level, not for nested lists
	if indent == 0 {
		h++
	}
	return h
}

func (l *listBlock) drawAtIndent(w term.Writer, y, indent int) int {
	if l.w <= indent+listIndent {
		return 0
	}

	startY := y
	effectiveWidth := l.w - indent - listIndent

	for i, item := range l.items {
		bullet := l.getBullet(i, item)

		for j, r := range bullet {
			w.SetCell(term.Coordinates{X: indent + j, Y: y}, term.NewCell(r, 1, l.cfg.Paragraph))
		}

		lines := wrapTextRun(item.content, effectiveWidth)
		for lineIdx, line := range lines {
			x := indent + listIndent
			for _, sp := range line {
				attr := resolveStyle(sp.style, l.cfg)
				for _, r := range sp.text {
					if x >= l.w {
						break
					}
					w.SetCell(term.Coordinates{X: x, Y: y + lineIdx}, term.NewCell(r, 1, attr))
					x++
				}
			}
		}
		y += len(lines)
		if len(lines) == 0 {
			y++
		}

		if item.nested != nil {
			y += item.nested.drawAtIndent(w, y, indent+listIndent)
		}
	}

	drawnHeight := y - startY
	// Only count spacing at top level
	if indent == 0 {
		drawnHeight++
	}
	return drawnHeight
}

func (l *listBlock) dimensionsAtIndent(indent int) (width, height int) {
	maxWidth := 0
	totalHeight := 0

	for _, item := range l.items {
		itemWidth := indent + listIndent + item.content.Len()
		if itemWidth > maxWidth {
			maxWidth = itemWidth
		}
		totalHeight++

		if item.nested != nil {
			nestedW, nestedH := item.nested.dimensionsAtIndent(indent + listIndent)
			if nestedW > maxWidth {
				maxWidth = nestedW
			}
			totalHeight += nestedH
		}
	}

	// Only add spacing at top level
	if indent == 0 {
		totalHeight++
	}
	return maxWidth, totalHeight
}

func (l *listBlock) getBullet(index int, item listItem) string {
	if item.isTask {
		if item.checked {
			return string(l.cfg.TaskListChecked) + " "
		}
		return string(l.cfg.TaskListUnchecked) + " "
	}

	if l.ordered {
		return fmt.Sprintf("%d.", l.start+index)
	}

	return string(l.cfg.ListBullet) + " "
}

func (l *listBlock) spanAtIndent(x, y, indent int) (text, url string, ok bool) {
	if l.w <= indent+listIndent {
		return
	}

	effectiveWidth := l.w - indent - listIndent
	currentY := 0

	for _, item := range l.items {
		lines := wrapTextRun(item.content, effectiveWidth)
		lineCount := len(lines)
		if lineCount == 0 {
			lineCount = 1
		}

		if y >= currentY && y < currentY+lineCount {
			lineIdx := y - currentY
			if lineIdx < len(lines) {
				adjustedX := x - indent - listIndent
				if adjustedX >= 0 {
					return spanAtInLine(lines[lineIdx], adjustedX)
				}
			}
			return
		}
		currentY += lineCount

		if item.nested != nil {
			nestedHeight := item.nested.heightAtIndent(l.w, indent+listIndent)
			if y >= currentY && y < currentY+nestedHeight {
				return item.nested.spanAtIndent(x, y-currentY, indent+listIndent)
			}
			currentY += nestedHeight
		}
	}
	return
}

func (l *listBlock) charAtIndent(x, y, indent int) (rune, bool) {
	if l.w <= indent+listIndent {
		return 0, false
	}

	effectiveWidth := l.w - indent - listIndent
	currentY := 0

	for i, item := range l.items {
		lines := wrapTextRun(item.content, effectiveWidth)
		lineCount := len(lines)
		if lineCount == 0 {
			lineCount = 1
		}

		if y >= currentY && y < currentY+lineCount {
			lineIdx := y - currentY
			bullet := l.getBullet(i, item)
			if lineIdx == 0 && x >= indent && x < indent+len(bullet) {
				bulletIdx := x - indent
				if bulletIdx < len(bullet) {
					return rune(bullet[bulletIdx]), true
				}
			}
			if lineIdx < len(lines) {
				adjustedX := x - indent - listIndent
				if adjustedX >= 0 {
					return charAtInLine(lines[lineIdx], adjustedX)
				}
			}
			return 0, false
		}
		currentY += lineCount

		if item.nested != nil {
			nestedHeight := item.nested.heightAtIndent(l.w, indent+listIndent)
			if y >= currentY && y < currentY+nestedHeight {
				return item.nested.charAtIndent(x, y-currentY, indent+listIndent)
			}
			currentY += nestedHeight
		}
	}
	return 0, false
}
