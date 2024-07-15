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

	"golang.org/x/image/math/fixed"
)

var (
	colorFill           = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	colorFillAlphaStep1 = color.RGBA{R: 192, G: 192, B: 192, A: 192}
	colorFillAlphaStep2 = color.RGBA{R: 128, G: 128, B: 128, A: 128}
	colorFillAlphaStep3 = color.RGBA{R: 64, G: 64, B: 64, A: 64}
)

// Shades: '░', '▒', '▓', '█'.
func (m *custom) shadeGlyph(dot fixed.Point26_6, c color.RGBA) (
	dr image.Rectangle, mask image.Image,
	maskp image.Point, advance fixed.Int26_6, ok bool,
) {
	dr = image.Rect(
		dot.X.Floor(),
		(dot.Y - m.offsetY).Floor(),
		(dot.X + m.width).Floor(),
		(dot.Y + m.height - m.offsetY).Floor(),
	)

	width := int(m.width)
	height := int(m.height)

	nPixels := width * height
	if cap(m.mask.Pix) < nPixels {
		m.mask.Pix = make([]uint8, 2*nPixels)
	}
	m.mask.Pix = m.mask.Pix[:nPixels]
	m.mask.Stride = width
	m.mask.Rect.Min.X = 0
	m.mask.Rect.Min.Y = 0
	m.mask.Rect.Max.X = width
	m.mask.Rect.Max.Y = height

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			m.mask.Set(x, y, c)
		}
	}
	mask = &m.mask
	maskp = mask.Bounds().Min
	advance = fixed.I(int(m.width))
	ok = true
	return
}
