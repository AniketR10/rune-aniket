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
			for _, r := range sp.text {
				if x >= p.w {
					break
				}
				w.SetCell(term.Coordinates{X: x, Y: i}, term.Cell{
					Ch:         r,
					Width:      1,
					Attributes: attr,
				})
				x++
			}
		}
	}
}

func (p *paragraphBlock) Dimensions() (width, height int) {
	return p.content.Len(), 2
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
