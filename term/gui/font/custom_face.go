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
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

var _ font.Face = (*custom)(nil)

// handles special runes used to build (T)UIs for
// pixel perfect rendering.
type custom struct {
	width, height fixed.Int26_6
	face          font.Face
	offsetY       fixed.Int26_6
	boldFont      bool

	// re-use allocs
	mask *ebiten.Image
}

func newCustomFace(width, height, offsetY float64, standard font.Face, bold bool) *custom {
	mask := ebiten.NewImage(int(math.Max(width, 1)), int(math.Max(height, 1)))
	return &custom{
		boldFont: bold,
		mask:     mask,
		offsetY:  float64ToFixed(offsetY),
		face:     standard,
		width:    float64ToFixed(width),
		height:   float64ToFixed(height),
	}
}

func (m *custom) Glyph(dot fixed.Point26_6, r rune) (
	dr image.Rectangle, mask image.Image,
	maskp image.Point, advance fixed.Int26_6, ok bool,
) {
	m.mask.Clear()

	dr = image.Rect(
		dot.X.Floor(),
		(dot.Y - m.offsetY).Floor(),
		(dot.X + m.width).Floor(),
		(dot.Y + m.height - m.offsetY).Floor(),
	)

	switch r {
	case '░':
		ok = m.shadeGlyph(dr, colorFillAlphaStep3)
	case '▒':
		ok = m.shadeGlyph(dr, colorFillAlphaStep2)
	case '▓':
		ok = m.shadeGlyph(dr, colorFillAlphaStep1)
	case '█':
		ok = m.shadeGlyph(dr, colorFill)
	case '─':
		ok = m.plusGlyph(dr, styleSingle, styleSingle, 0, 0)
	case '━':
		ok = m.plusGlyph(dr, styleBold, styleBold, 0, 0)
	case '╴':
		ok = m.plusGlyph(dr, styleSingle, 0, 0, 0)
	case '╶':
		ok = m.plusGlyph(dr, 0, styleSingle, 0, 0)
	case '╸':
		ok = m.plusGlyph(dr, styleBold, 0, 0, 0)
	case '╺':
		ok = m.plusGlyph(dr, 0, styleBold, 0, 0)
	case '│':
		ok = m.plusGlyph(dr, 0, 0, styleSingle, styleSingle)
	case '┃':
		ok = m.plusGlyph(dr, 0, 0, styleBold, styleBold)
	case '╵':
		ok = m.plusGlyph(dr, 0, 0, styleSingle, 0)
	case '╷':
		ok = m.plusGlyph(dr, 0, 0, 0, styleSingle)
	case '╹':
		ok = m.plusGlyph(dr, 0, 0, styleBold, 0)
	case '╻':
		ok = m.plusGlyph(dr, 0, 0, 0, styleBold)
	case '┌':
		ok = m.plusGlyph(dr, 0, styleSingle, 0, styleSingle)
	case '┍':
		ok = m.plusGlyph(dr, 0, styleBold, 0, styleSingle)
	case '┎':
		ok = m.plusGlyph(dr, 0, styleSingle, 0, styleBold)
	case '┏':
		ok = m.plusGlyph(dr, 0, styleBold, 0, styleBold)
	case '┐':
		ok = m.plusGlyph(dr, styleSingle, 0, 0, styleSingle)
	case '┑':
		ok = m.plusGlyph(dr, styleBold, 0, 0, styleSingle)
	case '┒':
		ok = m.plusGlyph(dr, styleSingle, 0, 0, styleBold)
	case '┓':
		ok = m.plusGlyph(dr, styleBold, 0, 0, styleBold)
	case '└':
		ok = m.plusGlyph(dr, 0, styleSingle, styleSingle, 0)
	case '┕':
		ok = m.plusGlyph(dr, 0, styleBold, styleSingle, 0)
	case '┖':
		ok = m.plusGlyph(dr, 0, styleSingle, styleBold, 0)
	case '┗':
		ok = m.plusGlyph(dr, 0, styleBold, styleBold, 0)
	case '┘':
		ok = m.plusGlyph(dr, styleSingle, 0, styleSingle, 0)
	case '┙':
		ok = m.plusGlyph(dr, styleBold, 0, styleSingle, 0)
	case '┚':
		ok = m.plusGlyph(dr, styleSingle, 0, styleBold, 0)
	case '┛':
		ok = m.plusGlyph(dr, styleBold, 0, styleBold, 0)
	case '├':
		ok = m.plusGlyph(dr, 0, styleSingle, styleSingle, styleSingle)
	case '┝':
		ok = m.plusGlyph(dr, 0, styleSingle, styleSingle, styleBold)
	case '┞':
		ok = m.plusGlyph(dr, 0, styleSingle, styleBold, styleSingle)
	case '┟':
		ok = m.plusGlyph(dr, 0, styleSingle, styleSingle, styleBold)
	case '┠':
		ok = m.plusGlyph(dr, 0, styleSingle, styleBold, styleBold)
	case '┡':
		ok = m.plusGlyph(dr, 0, styleBold, styleBold, styleSingle)
	case '┢':
		ok = m.plusGlyph(dr, 0, styleBold, styleSingle, styleBold)
	case '┣':
		ok = m.plusGlyph(dr, 0, styleBold, styleBold, styleBold)
	case '┤':
		ok = m.plusGlyph(dr, styleSingle, 0, styleSingle, styleSingle)
	case '┥':
		ok = m.plusGlyph(dr, styleBold, 0, styleSingle, styleSingle)
	case '┦':
		ok = m.plusGlyph(dr, styleSingle, 0, styleBold, styleSingle)
	case '┧':
		ok = m.plusGlyph(dr, styleSingle, 0, styleSingle, styleBold)
	case '┨':
		ok = m.plusGlyph(dr, styleSingle, 0, styleBold, styleBold)
	case '┩':
		ok = m.plusGlyph(dr, styleBold, 0, styleBold, styleSingle)
	case '┪':
		ok = m.plusGlyph(dr, styleBold, 0, styleSingle, styleBold)
	case '┫':
		ok = m.plusGlyph(dr, styleBold, 0, styleBold, styleBold)
	case '┬':
		ok = m.plusGlyph(dr, styleSingle, styleSingle, 0, styleSingle)
	case '┭':
		ok = m.plusGlyph(dr, styleBold, styleSingle, 0, styleSingle)
	case '┮':
		ok = m.plusGlyph(dr, styleSingle, styleBold, 0, styleSingle)
	case '┯':
		ok = m.plusGlyph(dr, styleBold, styleBold, 0, styleSingle)
	case '┰':
		ok = m.plusGlyph(dr, styleSingle, styleSingle, 0, styleBold)
	case '┱':
		ok = m.plusGlyph(dr, styleBold, styleSingle, 0, styleBold)
	case '┲':
		ok = m.plusGlyph(dr, styleSingle, styleBold, 0, styleBold)
	case '┳':
		ok = m.plusGlyph(dr, styleBold, styleBold, 0, styleBold)
	case '┴':
		ok = m.plusGlyph(dr, styleSingle, styleSingle, styleSingle, 0)
	case '┵':
		ok = m.plusGlyph(dr, styleBold, styleSingle, styleSingle, 0)
	case '┶':
		ok = m.plusGlyph(dr, styleSingle, styleBold, styleSingle, 0)
	case '┷':
		ok = m.plusGlyph(dr, styleBold, styleBold, styleSingle, 0)
	case '┸':
		ok = m.plusGlyph(dr, styleSingle, styleSingle, styleBold, 0)
	case '┹':
		ok = m.plusGlyph(dr, styleBold, styleSingle, styleBold, 0)
	case '┺':
		ok = m.plusGlyph(dr, styleSingle, styleBold, styleBold, 0)
	case '┻':
		ok = m.plusGlyph(dr, styleBold, styleBold, styleBold, 0)
	case '┼':
		ok = m.plusGlyph(dr, styleSingle, styleSingle, styleSingle, styleSingle)
	case '┽':
		ok = m.plusGlyph(dr, styleBold, styleSingle, styleSingle, styleSingle)
	case '┾':
		ok = m.plusGlyph(dr, styleSingle, styleBold, styleSingle, styleSingle)
	case '┿':
		ok = m.plusGlyph(dr, styleBold, styleBold, styleSingle, styleSingle)
	case '╀':
		ok = m.plusGlyph(dr, styleSingle, styleSingle, styleBold, styleSingle)
	case '╁':
		ok = m.plusGlyph(dr, styleSingle, styleSingle, styleSingle, styleBold)
	case '╂':
		ok = m.plusGlyph(dr, styleSingle, styleSingle, styleBold, styleBold)
	case '╃':
		ok = m.plusGlyph(dr, styleBold, styleSingle, styleBold, styleSingle)
	case '╄':
		ok = m.plusGlyph(dr, styleSingle, styleBold, styleBold, styleSingle)
	case '╅':
		ok = m.plusGlyph(dr, styleBold, styleSingle, styleSingle, styleBold)
	case '╆':
		ok = m.plusGlyph(dr, styleSingle, styleBold, styleSingle, styleBold)
	case '╇':
		ok = m.plusGlyph(dr, styleBold, styleBold, styleBold, styleSingle)
	case '╈':
		ok = m.plusGlyph(dr, styleBold, styleBold, styleSingle, styleBold)
	case '╉':
		ok = m.plusGlyph(dr, styleBold, styleSingle, styleBold, styleBold)
	case '╊':
		ok = m.plusGlyph(dr, styleSingle, styleBold, styleBold, styleBold)
	case '╋':
		ok = m.plusGlyph(dr, styleBold, styleBold, styleBold, styleBold)
	case '╼':
		ok = m.plusGlyph(dr, styleSingle, styleBold, 0, 0)
	case '╾':
		ok = m.plusGlyph(dr, styleBold, styleSingle, 0, 0)
	case '╽':
		ok = m.plusGlyph(dr, 0, 0, styleSingle, styleBold)
	case '╿':
		ok = m.plusGlyph(dr, 0, 0, styleBold, styleSingle)
	}

	if ok {
		mask = m.mask
		maskp = mask.Bounds().Min
		advance = fixed.I(int(m.width))
	}
	return
}

func (m *custom) GlyphBounds(r rune) (
	bounds fixed.Rectangle26_6, advance fixed.Int26_6, ok bool,
) {
	var dot fixed.Point26_6
	boundsRect, _, _, advance, ok := m.Glyph(dot, r)
	if !ok {
		return
	}

	bounds = fixedRectangleFromImageRectangle(boundsRect)
	return
}
func (m *custom) GlyphAdvance(r rune) (
	advance fixed.Int26_6, ok bool,
) {
	var dot fixed.Point26_6
	_, _, _, advance, ok = m.Glyph(dot, r)
	return
}

func (m *custom) Close() (ret error) {
	m.mask.Deallocate()
	// it's assumed that m.face is closed elsewhere
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
