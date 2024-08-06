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

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

var _ font.Face = (*custom)(nil)

// handles special runes used to build (T)UIs for
// pixel perfect rendering.
type custom struct {
	width, height fixed.Int26_6
	face          font.Face
	mask          image.RGBA
	offsetY       fixed.Int26_6
}

var (
	colorFill           = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	colorFillAlphaStep1 = color.RGBA{R: 192, G: 192, B: 192, A: 192}
	colorFillAlphaStep2 = color.RGBA{R: 128, G: 128, B: 128, A: 128}
	colorFillAlphaStep3 = color.RGBA{R: 64, G: 64, B: 64, A: 64}
)

func newCustomFace(width, height, offsetY float64, standard font.Face) *custom {
	return &custom{
		offsetY: float64ToFixed(offsetY),
		face:    standard,
		width:   float64ToFixed(width),
		height:  float64ToFixed(height),
	}
}

func (m *custom) Glyph(dot fixed.Point26_6, r rune) (
	dr image.Rectangle, mask image.Image,
	maskp image.Point, advance fixed.Int26_6, ok bool,
) {
	switch r {
	case '\u2588':
		dr, mask, maskp, advance, ok = m.shadeGlyph(dot, colorFill)
	case '\u2591':
		dr, mask, maskp, advance, ok = m.shadeGlyph(dot, colorFillAlphaStep3)
	case '\u2592':
		dr, mask, maskp, advance, ok = m.shadeGlyph(dot, colorFillAlphaStep2)
	case '\u2593':
		dr, mask, maskp, advance, ok = m.shadeGlyph(dot, colorFillAlphaStep1)
	default:
		return
	}
	return
}

func (m *custom) GlyphBounds(r rune) (
	bounds fixed.Rectangle26_6, advance fixed.Int26_6, ok bool,
) {
	var dot fixed.Point26_6
	var boundsRect image.Rectangle
	switch r {
	case '\u2588':
		boundsRect, _, _, advance, ok = m.shadeGlyph(dot, colorFill)
	case '\u2591':
		boundsRect, _, _, advance, ok = m.shadeGlyph(dot, colorFillAlphaStep3)
	case '\u2592':
		boundsRect, _, _, advance, ok = m.shadeGlyph(dot, colorFillAlphaStep2)
	case '\u2593':
		boundsRect, _, _, advance, ok = m.shadeGlyph(dot, colorFillAlphaStep1)
	default:
		return
	}

	bounds = fixedRectangleFromImageRectangle(boundsRect)
	return
}
func (m *custom) GlyphAdvance(r rune) (
	advance fixed.Int26_6, ok bool,
) {
	var dot fixed.Point26_6
	switch r {
	case '\u2588':
		_, _, _, advance, ok = m.shadeGlyph(dot, colorFill)
	case '\u2591':
		_, _, _, advance, ok = m.shadeGlyph(dot, colorFillAlphaStep3)
	case '\u2592':
		_, _, _, advance, ok = m.shadeGlyph(dot, colorFillAlphaStep2)
	case '\u2593':
		_, _, _, advance, ok = m.shadeGlyph(dot, colorFillAlphaStep1)
	default:
		return
	}
	return
}

func (m *custom) Close() (ret error) {
	// do nothing, it's assumed that m.face is closed elsewhere
	return
}

func (m *custom) Kern(r0, r1 rune) fixed.Int26_6 {
	return m.face.Kern(r0, r1)
}

func (m *custom) Metrics() font.Metrics {
	return m.face.Metrics()
}

func fixedRectangleFromImageRectangle(r image.Rectangle) fixed.Rectangle26_6 {
	return fixed.Rectangle26_6{
		Min: fixed.Point26_6{
			X: fixed.I(r.Min.X),
			Y: fixed.I(r.Min.Y),
		},
		Max: fixed.Point26_6{
			X: fixed.I(r.Max.X),
			Y: fixed.I(r.Max.Y),
		},
	}
}

func float64ToFixed(x float64) fixed.Int26_6 {
	return fixed.Int26_6(x * (1 << 6))
}
