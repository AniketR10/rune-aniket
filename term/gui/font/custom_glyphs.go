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

	"github.com/hajimehoshi/ebiten/v2"
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

	m.drawHorizontalLine(bounds, m.mask, float64(bounds.Min.X), yh, lhsize, lhstroke)
	m.drawHorizontalLine(bounds, m.mask, xh, yh, rhsize, rhstroke)
	m.drawVerticalLine(bounds, m.mask, xv, float64(bounds.Min.Y), tvsize, tvstroke)
	m.drawVerticalLine(bounds, m.mask, xv, yv, bvsize, bvstroke)

	ok = true
	return
}

func (m *custom) shadeGlyph(bounds image.Rectangle, c color.RGBA) (ok bool) {
	width := float64(bounds.Max.X - bounds.Min.X)
	height := float64(bounds.Max.Y - bounds.Min.Y)
	x := float64(bounds.Min.X)
	y := float64(bounds.Min.Y)

	m.drawRect(bounds, m.mask, x, y, width, height, c)
	ok = true
	return
}

func (m *custom) arcPlusGlyph(
	bounds image.Rectangle,
	topLeft, topRight, bottomLeft, bottomRight style,
) (ok bool) {
	if topRight == styleSingle {
		m.drawArcPlusGlyphTopRight(m.mask, bounds)
		ok = true
		return
	}

	if topLeft == styleSingle {
		m.drawArcPlusGlyphTopLeft(m.mask, bounds)
		ok = true
		return
	}

	if bottomRight == styleSingle {
		m.drawArcPlusGlyphBottomRight(bounds)
		ok = true
		return
	}

	if bottomLeft == styleSingle {
		m.drawArcPlusGlyphBottomLeft(bounds)
		ok = true
		return
	}

	return
}

func (m *custom) drawArcPlusGlyphBottomLeft(bounds image.Rectangle) {
	m.drawArcPlusGlyphTopLeft(m.altMask, bounds)

	var opt ebiten.DrawImageOptions
	opt.GeoM.Scale(1, -1)
	opt.GeoM.Translate(0, float64(bounds.Max.Y-bounds.Min.Y))
	m.mask.DrawImage(m.altMask, &opt)
}

func (m *custom) drawArcPlusGlyphBottomRight(bounds image.Rectangle) {
	m.drawArcPlusGlyphTopRight(m.altMask, bounds)

	var opt ebiten.DrawImageOptions
	opt.GeoM.Scale(1, -1)
	opt.GeoM.Translate(0, float64(bounds.Max.Y-bounds.Min.Y))
	m.mask.DrawImage(m.altMask, &opt)
}

func (m *custom) drawArcPlusGlyphTopRight(image *ebiten.Image, bounds image.Rectangle) {
	strokeWidth := m.strokeWidthFromMask(bounds, styleSingle)

	xv := centerX(bounds)
	yh := centerY(bounds)
	vBoundsFrom := xv
	vBoundsTo := xv + float64(strokeWidth)
	hBoundsFrom := yh
	hBoundsTo := yh + float64(strokeWidth)

	rx1, ry1 := vBoundsFrom, hBoundsFrom
	for ; rx1 < vBoundsTo && ry1 < hBoundsTo; rx1, ry1 = rx1+1, ry1+1 {
		rx2 := rx1 * rx1
		ry2 := ry1 * ry1
		quarter := rx2 / math.Sqrt(rx2+ry2)

		for x := float64(bounds.Min.X); x < quarter; x++ {
			y := ry1 * math.Sqrt(1-x*x/rx2)
			x := float64(bounds.Min.X) + float64(bounds.Max.X) -
				(x + 1 - float64(bounds.Min.X))
			y = math.Min(math.Max(y, float64(bounds.Min.Y)), hBoundsTo-1)
			x = centerToX(x, bounds, strokeWidth)
			y = centerFromY(y, bounds, strokeWidth)
			image.Set(int(x), int(y), colorFill)
		}

		quarter = ry2 / math.Sqrt(rx2+ry2)
		for y := float64(bounds.Min.Y); y < quarter; y++ {
			x := rx1 * math.Sqrt(1-y*y/ry2)
			y := math.Min(math.Max(y, float64(bounds.Min.Y)), ry1)
			x = float64(bounds.Max.X) - (x + 1 - float64(bounds.Min.X))
			x = centerToX(x, bounds, strokeWidth)
			y = centerFromY(y, bounds, strokeWidth)
			image.Set(int(x), int(y), colorFill)
		}
	}

	// ensure the part closer to the edge of the cell is filled
	m.drawVerticalLine(bounds, image, xv, float64(bounds.Min.Y),
		math.Max(1, float64(strokeWidth)/2), strokeWidth)
}

func (m *custom) drawArcPlusGlyphTopLeft(image *ebiten.Image, bounds image.Rectangle) {
	strokeWidth := m.strokeWidthFromMask(bounds, styleSingle)

	xv := centerX(bounds)
	yh := centerY(bounds)
	vBoundsFrom := centerFromX(xv, bounds, strokeWidth)
	vBoundsTo := centerToX(xv, bounds, strokeWidth)
	hBoundsFrom := centerFromY(yh, bounds, strokeWidth)
	hBoundsTo := centerToY(yh, bounds, strokeWidth)

	rx1, ry1 := vBoundsFrom, hBoundsFrom
	for ; rx1 < vBoundsTo && ry1 < hBoundsTo; rx1, ry1 = rx1+1, ry1+1 {
		rx2 := rx1 * rx1
		ry2 := ry1 * ry1
		quarter := rx2 / math.Sqrt(rx2+ry2)

		for x := float64(bounds.Min.X); x < quarter; x++ {
			y := ry1 * math.Sqrt(1-x*x/rx2)
			x := math.Min(math.Max(x+1, float64(bounds.Min.X)), rx1)
			y = math.Min(math.Max(y, float64(bounds.Min.Y)), hBoundsTo-1)
			image.Set(int(x), int(y), colorFill)
		}

		quarter = ry2 / math.Sqrt(rx2+ry2)
		for y := float64(bounds.Min.Y); y < quarter; y++ {
			x := rx1 * math.Sqrt(1-y*y/ry2)
			y := math.Min(math.Max(y, float64(bounds.Min.Y)), ry1)
			x = math.Min(math.Max(x+1, float64(bounds.Min.X)), vBoundsTo-1)
			image.Set(int(x), int(y), colorFill)
		}
	}

	// ensure the part closer to the edge of the cell is filled
	m.drawHorizontalLine(bounds, image, float64(bounds.Min.X), yh,
		math.Max(1, float64(strokeWidth)/2), strokeWidth)
	m.drawVerticalLine(bounds, image, xv, float64(bounds.Min.Y),
		math.Max(1, float64(strokeWidth)/2), strokeWidth)
}

func (m *custom) dottedHorizontalGlyph(
	bounds image.Rectangle, s style, gaps int,
) (ok bool) {
	strokeWidth := m.strokeWidthFromMask(bounds, s)
	dashGapLen := float64(m.strokeWidthFromMask(bounds, styleSingle))

	width := float64(bounds.Max.X - bounds.Min.X)
	dashLen := math.Trunc(math.Max(
		float64(width-dashGapLen*float64(gaps+1))/float64(gaps+1),
		1,
	))
	y := centerY(bounds)
	for gap := 0.; gap < float64(gaps+1); gap++ {
		x := float64(bounds.Min.X) + math.Trunc(
			math.Min(
				float64(gap*(dashLen+dashGapLen)),
				float64(bounds.Max.X)-1,
			),
		)
		if int(gap) == gaps {
			// add or reduce pixels to the last dash length such that gaps are always
			// equal, which gives a more visually pleasing result
			dashLen = float64(bounds.Max.X) - dashGapLen - x
		}
		m.drawHorizontalLine(bounds, m.mask, x, y, float64(dashLen), strokeWidth)
	}
	ok = true
	return
}

func (m *custom) dottedVerticalGlyph(
	bounds image.Rectangle, s style, gaps int,
) (ok bool) {
	strokeWidth := m.strokeWidthFromMask(bounds, s)
	dashGapLen := float64(m.strokeWidthFromMask(bounds, styleBold))

	height := float64(bounds.Max.Y - bounds.Min.Y)
	dashLen := math.Trunc(math.Max(
		float64(height-dashGapLen*float64(gaps+1))/float64(gaps+1),
		1,
	))
	x := centerX(bounds)
	for gap := 0.; gap < float64(gaps+1); gap++ {
		y := float64(bounds.Min.Y) + math.Trunc(
			math.Min(
				float64(gap*(dashLen+dashGapLen)),
				float64(bounds.Max.Y)-1,
			),
		)
		if int(gap) == gaps {
			// add or reduce pixels to the last dash length such that gaps are always
			// equal, which gives a more visually pleasing result
			dashLen = float64(bounds.Max.Y) - dashGapLen - y
		}
		m.drawVerticalLine(bounds, m.mask, x, y, float64(dashLen), strokeWidth)
	}
	ok = true
	return
}

func (m *custom) drawVerticalLine(
	bounds image.Rectangle, image *ebiten.Image,
	x, y, size float64, width int,
) {
	startx := centerFromX(x, bounds, width)
	endx := centerToX(x, bounds, width)
	m.drawRect(bounds, image, startx, y, endx-startx, size, colorFill)
}

func (m *custom) drawHorizontalLine(
	bounds image.Rectangle, image *ebiten.Image,
	x, y, size float64, width int) {
	starty := centerFromY(y, bounds, width)
	endy := centerToY(y, bounds, width)
	m.drawRect(bounds, image, x, starty, size, endy-starty, colorFill)
}

func (m *custom) drawRect(
	bounds image.Rectangle, image *ebiten.Image,
	x, y, width, height float64, color color.RGBA,
) {
	startx := int(x)
	endx := int(math.Min(x+width-1, float64(bounds.Max.X-1)))

	starty := int(y)
	endy := int(math.Min(y+height-1, float64(bounds.Max.Y-1)))

	for y := starty; y <= endy; y++ {
		for x := startx; x <= endx; x++ {
			image.Set(x, y, color)
		}
	}
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
	// bounds.Min.Y could be negative
	return float64(bounds.Min.Y) + float64(bounds.Max.Y-bounds.Min.Y)/2
}

func centerX(bounds image.Rectangle) float64 {
	// bounds.Min.X could be negative
	return float64(bounds.Min.X) + float64(bounds.Max.X-bounds.Min.X)/2
}

func centerFromX(x float64, bounds image.Rectangle, strokeWidth int) float64 {
	return math.Max(
		math.Trunc(x-float64(strokeWidth)/2),
		float64(bounds.Min.X),
	)
}

func centerFromY(y float64, bounds image.Rectangle, strokeWidth int) float64 {
	return math.Max(
		math.Trunc(y-float64(strokeWidth)/2),
		float64(bounds.Min.Y),
	)
}

func centerToX(x float64, bounds image.Rectangle, strokeWidth int) float64 {
	return math.Min(
		math.Trunc(x+float64(strokeWidth)/2),
		float64(bounds.Max.X-1),
	)
}

func centerToY(y float64, bounds image.Rectangle, strokeWidth int) float64 {
	return math.Min(
		math.Trunc(y+float64(strokeWidth)/2),
		float64(bounds.Max.Y-1),
	)
}
