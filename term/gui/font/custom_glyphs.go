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
	"image"
	"image/color"
	"math"
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
	bounds image.Rectangle,
	leftHorizontal, rightHorizontal, topVertical, bottomVertical style,
) (ok bool) {
	lhstroke := m.strokeWidthFromMask(bounds, leftHorizontal)
	rhstroke := m.strokeWidthFromMask(bounds, rightHorizontal)
	tvstroke := m.strokeWidthFromMask(bounds, topVertical)
	bvstroke := m.strokeWidthFromMask(bounds, bottomVertical)

	xv, yh, xh, yv, lhsize, rhsize, tvsize, bvsize := plusGlyphBounds(
		bounds, lhstroke, rhstroke, tvstroke, bvstroke)

	m.drawHorizontalLine(bounds, float64(bounds.Min.X), yh, lhsize, lhstroke)
	m.drawHorizontalLine(bounds, xh, yh, rhsize, rhstroke)
	m.drawVerticalLine(bounds, xv, float64(bounds.Min.Y), tvsize, tvstroke)
	m.drawVerticalLine(bounds, xv, yv, bvsize, bvstroke)

	ok = true
	return
}

func (m *custom) shadeGlyph(bounds image.Rectangle, c color.RGBA) (ok bool) {
	width := float64(bounds.Max.X - bounds.Min.X)
	height := float64(bounds.Max.Y - bounds.Min.Y)
	x := float64(bounds.Min.X)
	y := float64(bounds.Min.Y)

	m.drawRect(bounds, x, y, width, height, c)
	ok = true
	return
}

func (m *custom) drawVerticalLine(bounds image.Rectangle, x, y, size float64, width int) {
	startx := centerFromX(x, bounds, width)
	endx := centerToX(x, bounds, width)
	m.drawRect(bounds, startx, y, endx-startx, size, colorFill)
}

func (m *custom) drawHorizontalLine(bounds image.Rectangle, x, y, size float64, width int) {
	starty := centerFromY(y, bounds, width)
	endy := centerToY(y, bounds, width)
	m.drawRect(bounds, x, starty, size, endy-starty, colorFill)
}

func (m *custom) drawRect(bounds image.Rectangle, x, y, width, height float64, color color.RGBA) {
	startx := int(x)
	endx := int(math.Min(x+width, float64(bounds.Max.X)))

	starty := int(y)
	endy := int(math.Min(y+height, float64(bounds.Max.Y)))

	for y := starty; y < endy; y++ {
		for x := startx; x < endx; x++ {
			m.mask.Set(x, y, color)
		}
	}
}

func plusGlyphBounds(
	bounds image.Rectangle,
	lhstroke, rhstroke, tvstroke, bvstroke int,
) (xv, yh, xh, yv, lhsize, rhsize, tvsize, bvsize float64) {

	xv = centerX(bounds)
	yh = centerY(bounds)

	tvBoundsFrom := centerFromX(xv, bounds, tvstroke)
	tvBoundsTo := centerToX(xv, bounds, tvstroke)

	bvBoundsFrom := centerFromX(xv, bounds, bvstroke)
	bvBoundsTo := centerToX(xv, bounds, bvstroke)

	lhBoundsFrom := centerFromY(yh, bounds, lhstroke)
	lhBoundsTo := centerToY(yh, bounds, lhstroke)

	rhBoundsFrom := centerFromY(yh, bounds, rhstroke)
	rhBoundsTo := centerToY(yh, bounds, rhstroke)

	lhsize = math.Max(tvBoundsTo, bvBoundsTo) - float64(bounds.Min.X)
	xh = math.Min(tvBoundsFrom, bvBoundsFrom)
	rhsize = float64(bounds.Max.X-bounds.Min.X) - xh + float64(bounds.Min.X)

	tvsize = math.Max(lhBoundsTo, rhBoundsTo) - float64(bounds.Min.Y)
	yv = math.Min(lhBoundsFrom, rhBoundsFrom)
	bvsize = float64(bounds.Max.Y-bounds.Min.Y) - yv + float64(bounds.Min.Y)

	return
}

func centerY(bounds image.Rectangle) float64 {
	// bounds.Min.X could be negative
	return float64(bounds.Min.Y) + float64(bounds.Max.Y-bounds.Min.Y)/2
}

func centerX(bounds image.Rectangle) float64 {
	// bounds.Min.Y could be negative
	return float64(bounds.Min.X) + float64(bounds.Max.X-bounds.Min.X)/2
}

func centerFromX(x float64, bounds image.Rectangle, strokeWidth int) float64 {
	return math.Max(math.Trunc(x-float64(strokeWidth)/2), float64(bounds.Min.X))
}

func centerFromY(y float64, bounds image.Rectangle, strokeWidth int) float64 {
	return math.Max(math.Trunc(y-float64(strokeWidth)/2), float64(bounds.Min.Y))
}

func centerToX(x float64, bounds image.Rectangle, strokeWidth int) float64 {
	return math.Min(math.Trunc(x+float64(strokeWidth)/2), float64(bounds.Max.X))
}

func centerToY(y float64, bounds image.Rectangle, strokeWidth int) float64 {
	return math.Min(math.Trunc(y+float64(strokeWidth)/2), float64(bounds.Max.Y))
}

func (m *custom) strokeWidthFromMask(bounds image.Rectangle, mask style) int {
	// 1/8 of the cell for standard stroke width, if font is bold, then 1/6
	factor := 8.0
	if m.boldFont {
		factor = 6.0
	}
	width := float64(bounds.Max.X - bounds.Min.X)
	switch mask {
	case styleSingle:
		return int(math.Max(math.Trunc(width/factor), 1))
	case styleBold:
		// double for bold stroke
		return int(math.Max(math.Trunc(width/factor), 1)) * 2
	default:
		return 0
	}
}
