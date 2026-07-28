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
		w.SetCell(term.Coordinates{X: x, Y: 0}, term.Cell{
			Ch:         ch,
			Width:      1,
			Attributes: attr,
		})
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
