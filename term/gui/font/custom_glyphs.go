// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package font

import (
	"image/color"
	"math"

	"golang.org/x/image/math/fixed"
	"unstable.build/go-tui/term/gui/drawrect"
)

type style uint8

const (
	styleNone style = iota
	styleSingle
	styleBold
)

var (
	colorFill           = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	colorFillAlphaStep1 = color.RGBA{R: 192, G: 192, B: 192, A: 192}
	colorFillAlphaStep2 = color.RGBA{R: 128, G: 128, B: 128, A: 128}
	colorFillAlphaStep3 = color.RGBA{R: 64, G: 64, B: 64, A: 64}
)

func (m *custom) plusGlyph(
	dot fixed.Point26_6,
	leftHorizontal, rightHorizontal, topVertical, bottomVertical style,
) (ok bool) {
	width := fixedToFloat64(m.width)
	height := fixedToFloat64(m.height)

	lhwidth := m.strokeWidthFromMask(leftHorizontal)
	rhwidth := m.strokeWidthFromMask(rightHorizontal)
	tvwidth := m.strokeWidthFromMask(topVertical)
	bvwidth := m.strokeWidthFromMask(bottomVertical)

	m.drawRect(0, math.Max(height/2-lhwidth/2, 1), width/2, lhwidth, colorFill)
	m.drawRect(width/2, math.Max(height/2-rhwidth/2, 1), width/2, rhwidth, colorFill)
	m.drawRect(math.Max(width/2-tvwidth/2, 1), 0, tvwidth, height/2, colorFill)
	m.drawRect(math.Max(width/2-bvwidth/2, 1), height/2, bvwidth, height/2, colorFill)

	ok = true
	return
}

func (m *custom) shadeGlyph(dot fixed.Point26_6, c color.RGBA) (ok bool) {
	width := fixedToFloat64(m.width)
	height := fixedToFloat64(m.height)

	m.drawRect(0, 0, width, height, c)
	ok = true
	return
}

func (m *custom) drawRect(x, y, width, height float64, color color.RGBA) {
	m.vs, m.is = drawrect.DrawRect(&m.path, m.vs, m.is, m.mask,
		float32(x), float32(y), float32(width), float32(height), color, false)
}

func (m *custom) strokeWidthFromMask(mask style) float64 {
	switch mask {
	case styleSingle:
		return float64(m.strokeWidth)
	case styleBold:
		return float64(m.boldStrokeWidth)
	default:
		return 0
	}
}
