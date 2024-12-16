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
	overlapX      int
	overlapY      int

	// re-use allocs
	mask    *ebiten.Image
	altMask *ebiten.Image
}

func newCustomFace(
	width, height, offsetY float64,
	standard font.Face, bold bool,
	overlapX, overlapY int,
) *custom {
	w := int(math.Max(width, 1))
	h := int(math.Max(height, 1))
	mask := ebiten.NewImage(w, h)
	altMask := ebiten.NewImage(w, h)
	return &custom{
		boldFont: bold,
		mask:     mask,
		altMask:  altMask,
		offsetY:  float64ToFixed(offsetY),
		face:     standard,
		width:    float64ToFixed(width),
		height:   float64ToFixed(height),
		// we want certain characters to take into consideration cell overlap pixels
		overlapX: overlapX,
		overlapY: overlapY,
	}
}

func (m *custom) Glyph(dot fixed.Point26_6, r rune) (
	dr image.Rectangle, mask image.Image,
	maskp image.Point, advance fixed.Int26_6, ok bool,
) {
	m.mask.Clear()
	m.altMask.Clear()

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
	case '╯':
		ok = m.arcPlusGlyph(dr, styleSingle, 0, 0, 0)
	case '╰':
		ok = m.arcPlusGlyph(dr, 0, styleSingle, 0, 0)
	case '╮':
		ok = m.arcPlusGlyph(dr, 0, 0, styleSingle, 0)
	case '╭':
		ok = m.arcPlusGlyph(dr, 0, 0, 0, styleSingle)
	case '┄':
		ok = m.dottedHorizontalGlyph(dr, styleSingle, 2)
	case '┅':
		ok = m.dottedHorizontalGlyph(dr, styleBold, 2)
	case '┈':
		ok = m.dottedHorizontalGlyph(dr, styleSingle, 3)
	case '┉':
		ok = m.dottedHorizontalGlyph(dr, styleBold, 3)
	case '╌':
		ok = m.dottedHorizontalGlyph(dr, styleSingle, 1)
	case '╍':
		ok = m.dottedHorizontalGlyph(dr, styleBold, 1)
	case '┆':
		ok = m.dottedVerticalGlyph(dr, styleSingle, 2)
	case '┇':
		ok = m.dottedVerticalGlyph(dr, styleBold, 2)
	case '┊':
		ok = m.dottedVerticalGlyph(dr, styleSingle, 3)
	case '┋':
		ok = m.dottedVerticalGlyph(dr, styleBold, 3)
	case '╎':
		ok = m.dottedVerticalGlyph(dr, styleSingle, 1)
	case '╏':
		ok = m.dottedVerticalGlyph(dr, styleBold, 1)
	case '▉':
		ok = m.blockGlyph(dr, [][]bool{{true, true, true, true, true, true, true, false}})
	case '▊':
		ok = m.blockGlyph(dr, [][]bool{{true, true, true, false}})
	case '▋':
		ok = m.blockGlyph(dr, [][]bool{{true, true, true, true, true, false, false, false}})
	case '▌':
		ok = m.blockGlyph(dr, [][]bool{{true, false}})
	case '▍':
		ok = m.blockGlyph(dr, [][]bool{{true, true, true, false, false, false, false, false}})
	case '▎':
		ok = m.blockGlyph(dr, [][]bool{{true, false, false, false}})
	case '▏':
		ok = m.blockGlyphOuterSquare(dr, 0, 0, styleSingle, 0)
	case '▔':
		ok = m.blockGlyphOuterSquare(dr, styleSingle, 0, 0, 0)
	case '▀':
		ok = m.blockGlyph(dr, [][]bool{{true}, {false}})
	case '▁':
		ok = m.blockGlyphOuterSquare(dr, 0, styleSingle, 0, 0)
	case '▂':
		ok = m.blockGlyph(dr, [][]bool{
			{false}, {false}, {false}, {true},
		})
	case '▃':
		ok = m.blockGlyph(dr, [][]bool{
			{false}, {false}, {false}, {false}, {false}, {true}, {true}, {true},
		})
	case '▄':
		ok = m.blockGlyph(dr, [][]bool{{false}, {true}})
	case '▅':
		ok = m.blockGlyph(dr, [][]bool{
			{false}, {false}, {false}, {true}, {true}, {true}, {true}, {true},
		})
	case '▆':
		ok = m.blockGlyph(dr, [][]bool{
			{false}, {true}, {true}, {true},
		})
	case '▇':
		ok = m.blockGlyph(dr, [][]bool{
			{false}, {true}, {true}, {true}, {true}, {true}, {true}, {true},
		})
	case '▐':
		ok = m.blockGlyph(dr, [][]bool{{false, true}})
	case '▕':
		ok = m.blockGlyphOuterSquare(dr, 0, 0, 0, styleSingle)
	case '▖':
		ok = m.blockGlyph(dr, [][]bool{{false, false}, {true, false}})
	case '▗':
		ok = m.blockGlyph(dr, [][]bool{{false, false}, {false, true}})
	case '▘':
		ok = m.blockGlyph(dr, [][]bool{{true, false}, {false, false}})
	case '▙':
		ok = m.blockGlyph(dr, [][]bool{{true, false}, {true, true}})
	case '▚':
		ok = m.blockGlyph(dr, [][]bool{{true, false}, {false, true}})
	case '▛':
		ok = m.blockGlyph(dr, [][]bool{{true, true}, {true, false}})
	case '▜':
		ok = m.blockGlyph(dr, [][]bool{{true, true}, {false, true}})
	case '▝':
		ok = m.blockGlyph(dr, [][]bool{{false, true}, {false, false}})
	case '▞':
		ok = m.blockGlyph(dr, [][]bool{{false, true}, {true, false}})
	case '▟':
		ok = m.blockGlyph(dr, [][]bool{{false, true}, {true, true}})

	// Some of Unicode 13 Symbols for Legacy Computing block
	// https://en.wikipedia.org/wiki/Symbols_for_Legacy_Computing
	case '\U0001fb00':
		ok = m.blockGlyph(dr, [][]bool{
			{true, false}, {false, false}, {false, false},
		})
	case '\U0001fb01':
		ok = m.blockGlyph(dr, [][]bool{
			{false, true}, {false, false}, {false, false},
		})
	case '\U0001fb02':
		ok = m.blockGlyph(dr, [][]bool{
			{false, true}, {false, false}, {false, false},
		})
	case '\U0001fb03':
		ok = m.blockGlyph(dr, [][]bool{
			{false, false}, {true, false}, {false, false},
		})
	case '\U0001fb04':
		ok = m.blockGlyph(dr, [][]bool{
			{true, false}, {true, false}, {false, false},
		})
	case '\U0001fb05':
		ok = m.blockGlyph(dr, [][]bool{
			{false, true}, {true, false}, {false, false},
		})
	case '\U0001fb06':
		ok = m.blockGlyph(dr, [][]bool{
			{true, true}, {true, false}, {false, false},
		})
	case '\U0001fb07':
		ok = m.blockGlyph(dr, [][]bool{
			{false, false}, {false, true}, {false, false},
		})
	case '\U0001fb08':
		ok = m.blockGlyph(dr, [][]bool{
			{true, false}, {false, true}, {false, false},
		})
	case '\U0001fb09':
		ok = m.blockGlyph(dr, [][]bool{
			{false, true}, {false, true}, {false, false},
		})
	case '\U0001fb0A':
		ok = m.blockGlyph(dr, [][]bool{
			{true, true}, {false, true}, {false, false},
		})
	case '\U0001fb0B':
		ok = m.blockGlyph(dr, [][]bool{
			{false, false}, {true, true}, {false, false},
		})
	case '\U0001fb0C':
		ok = m.blockGlyph(dr, [][]bool{
			{true, false}, {true, true}, {false, false},
		})
	case '\U0001fb0D':
		ok = m.blockGlyph(dr, [][]bool{
			{false, true}, {true, true}, {false, false},
		})
	case '\U0001fb0E':
		ok = m.blockGlyph(dr, [][]bool{
			{true, true}, {true, true}, {false, false},
		})
	case '\U0001fb0F':
		ok = m.blockGlyph(dr, [][]bool{
			{false, false}, {false, false}, {true, false},
		})
	case '\U0001fb10':
		ok = m.blockGlyph(dr, [][]bool{
			{true, false}, {false, false}, {true, false},
		})
	case '\U0001fb11':
		ok = m.blockGlyph(dr, [][]bool{
			{false, true}, {false, false}, {true, false},
		})
	case '\U0001fb12':
		ok = m.blockGlyph(dr, [][]bool{
			{false, true}, {false, false}, {true, false},
		})
	case '\U0001fb13':
		ok = m.blockGlyph(dr, [][]bool{
			{false, false}, {true, false}, {true, false},
		})
	case '\U0001fb14':
		ok = m.blockGlyph(dr, [][]bool{
			{true, false}, {true, false}, {true, false},
		})
	case '\U0001fb15':
		ok = m.blockGlyph(dr, [][]bool{
			{false, true}, {true, false}, {true, false},
		})
	case '\U0001fb16':
		ok = m.blockGlyph(dr, [][]bool{
			{true, true}, {true, false}, {true, false},
		})
	case '\U0001fb17':
		ok = m.blockGlyph(dr, [][]bool{
			{false, false}, {false, true}, {true, false},
		})
	case '\U0001fb18':
		ok = m.blockGlyph(dr, [][]bool{
			{true, false}, {false, true}, {true, false},
		})
	case '\U0001fb19':
		ok = m.blockGlyph(dr, [][]bool{
			{false, true}, {false, true}, {true, false},
		})
	case '\U0001fb1A':
		ok = m.blockGlyph(dr, [][]bool{
			{true, true}, {false, true}, {true, false},
		})
	case '\U0001fb1B':
		ok = m.blockGlyph(dr, [][]bool{
			{false, false}, {true, true}, {true, false},
		})
	case '\U0001fb1C':
		ok = m.blockGlyph(dr, [][]bool{
			{true, false}, {true, true}, {true, false},
		})
	case '\U0001fb1D':
		ok = m.blockGlyph(dr, [][]bool{
			{false, true}, {true, true}, {true, false},
		})
	case '\U0001fb1E':
		ok = m.blockGlyph(dr, [][]bool{
			{false, false}, {false, false}, {false, true},
		})
	case '\U0001fb1F':
		ok = m.blockGlyph(dr, [][]bool{
			{true, false}, {false, false}, {false, false},
		})
	case '\U0001fb20':
		ok = m.blockGlyph(dr, [][]bool{
			{false, true}, {false, false}, {false, true},
		})
	case '\U0001fb21':
		ok = m.blockGlyph(dr, [][]bool{
			{true, true}, {false, false}, {false, true},
		})
	case '\U0001fb22':
		ok = m.blockGlyph(dr, [][]bool{
			{false, false}, {true, false}, {false, true},
		})
	case '\U0001fb23':
		ok = m.blockGlyph(dr, [][]bool{
			{true, false}, {true, false}, {false, true},
		})
	case '\U0001fb24':
		ok = m.blockGlyph(dr, [][]bool{
			{false, true}, {true, false}, {false, true},
		})
	case '\U0001fb25':
		ok = m.blockGlyph(dr, [][]bool{
			{true, true}, {true, false}, {false, true},
		})
	case '\U0001fb26':
		ok = m.blockGlyph(dr, [][]bool{
			{false, false}, {false, true}, {false, true},
		})
	case '\U0001fb27':
		ok = m.blockGlyph(dr, [][]bool{
			{true, false}, {false, true}, {false, true},
		})
	case '\U0001fb28':
		ok = m.blockGlyph(dr, [][]bool{
			{true, true}, {false, true}, {false, true},
		})
	case '\U0001fb29':
		ok = m.blockGlyph(dr, [][]bool{
			{false, false}, {true, true}, {false, true},
		})
	case '\U0001fb2A':
		ok = m.blockGlyph(dr, [][]bool{
			{true, false}, {true, true}, {false, true},
		})
	case '\U0001fb2B':
		ok = m.blockGlyph(dr, [][]bool{
			{false, true}, {true, true}, {false, true},
		})
	case '\U0001fb2C':
		ok = m.blockGlyph(dr, [][]bool{
			{true, true}, {true, true}, {false, true},
		})
	case '\U0001fb2D':
		ok = m.blockGlyph(dr, [][]bool{
			{false, false}, {false, false}, {true, true},
		})
	case '\U0001fb2E':
		ok = m.blockGlyph(dr, [][]bool{
			{true, false}, {false, false}, {true, true},
		})
	case '\U0001fb2F':
		ok = m.blockGlyph(dr, [][]bool{
			{false, true}, {false, false}, {true, true},
		})
	case '\U0001fb30':
		ok = m.blockGlyph(dr, [][]bool{
			{true, true}, {false, false}, {true, true},
		})
	case '\U0001fb31':
		ok = m.blockGlyph(dr, [][]bool{
			{false, false}, {true, false}, {true, true},
		})
	case '\U0001fb32':
		ok = m.blockGlyph(dr, [][]bool{
			{true, false}, {true, false}, {true, true},
		})
	case '\U0001fb33':
		ok = m.blockGlyph(dr, [][]bool{
			{false, true}, {true, false}, {true, true},
		})
	case '\U0001fb34':
		ok = m.blockGlyph(dr, [][]bool{
			{true, true}, {true, false}, {true, true},
		})
	case '\U0001fb35':
		ok = m.blockGlyph(dr, [][]bool{
			{false, false}, {false, true}, {true, true},
		})
	case '\U0001fb36':
		ok = m.blockGlyph(dr, [][]bool{
			{true, false}, {false, true}, {true, true},
		})
	case '\U0001fb37':
		ok = m.blockGlyph(dr, [][]bool{
			{false, true}, {false, true}, {true, true},
		})
	case '\U0001fb38':
		ok = m.blockGlyph(dr, [][]bool{
			{true, true}, {false, true}, {true, true},
		})
	case '\U0001fb39':
		ok = m.blockGlyph(dr, [][]bool{
			{false, false}, {true, true}, {true, true},
		})
	case '\U0001fb3A':
		ok = m.blockGlyph(dr, [][]bool{
			{true, false}, {true, true}, {true, true},
		})
	case '\U0001fb3B':
		ok = m.blockGlyph(dr, [][]bool{
			{false, true}, {true, true}, {true, true},
		})
	case '\U0001fb7C':
		ok = m.blockGlyphOuterSquare(dr, 0, styleSingle, styleSingle, 0)
	case '\U0001fb7D':
		ok = m.blockGlyphOuterSquare(dr, styleSingle, 0, styleSingle, 0)
	case '\U0001fb7E':
		ok = m.blockGlyphOuterSquare(dr, styleSingle, 0, 0, styleSingle)
	case '\U0001fb7F':
		ok = m.blockGlyphOuterSquare(dr, 0, styleSingle, 0, styleSingle)
	case '\U0001fb80':
		ok = m.blockGlyphOuterSquare(dr, styleSingle, styleSingle, 0, 0)
	}

	if ok {
		mask = m.mask
		maskp = dr.Min
		advance = fixed.I(dr.Max.X - dr.Min.X)
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

func fixedToFloat64(x fixed.Int26_6) float64 {
	return float64(x>>6) + float64(x&((1<<6)-1))/float64(1<<6)
}
