// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package drawrect

import (
	"image"
	"image/color"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
)

var (
	whiteImage        *ebiten.Image
	whiteSubImage     *ebiten.Image
	whiteSubImageOnce sync.Once
)

var defaultDrawTrianglesOptions = ebiten.DrawTrianglesOptions{
	ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha,
	// NOTE: if you change this, you must manually test that white/light themes
	// look good, that the initial shader looks good, and that when opacity
	// is 0.4 or less things look ok.
	Blend: ebiten.Blend{
		BlendFactorSourceRGB:      ebiten.BlendFactorOne,
		BlendFactorDestinationRGB: ebiten.BlendFactorOneMinusSourceAlpha,
		BlendOperationRGB:         ebiten.BlendOperationAdd,
		BlendOperationAlpha:       ebiten.BlendOperationAdd,
	},
	AntiAlias: false,
}

// Init initializes this package. This function must be called before
// any of the other functions in this package
func Init() {
	whiteSubImageOnce.Do(func() {
		whiteImage = ebiten.NewImage(3, 3)
		whiteSubImage = whiteImage.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)

		b := whiteImage.Bounds()
		pix := make([]byte, 4*b.Dx()*b.Dy())
		for i := range pix {
			pix[i] = 0xff
		}
		// This is hacky, but WritePixels is better than Fill in term of automatic texture packing.
		whiteImage.WritePixels(pix)
	})
}

func drawVerticesForUtil(
	dst *ebiten.Image, vs []ebiten.Vertex,
	is []uint16, clr color.RGBA,
) {
	r, g, b, a := clr.RGBA()
	for i := range vs {
		vs[i].SrcX = 1
		vs[i].SrcY = 1
		vs[i].ColorR = float32(r) / 0xffff
		vs[i].ColorG = float32(g) / 0xffff
		vs[i].ColorB = float32(b) / 0xffff
		vs[i].ColorA = float32(a) / 0xffff
	}

	dst.DrawTriangles(vs, is, whiteSubImage, &defaultDrawTrianglesOptions)
}

// DrawStroke strokes a line (x0, y0)-(x1, y1) with the specified width and color.
//
// clr has be to be a solid (non-transparent) color.
func DrawStroke(
	path *Path, vs []ebiten.Vertex, is []uint16, dst *ebiten.Image,
	x0, y0, x1, y1 float32,
	strokeWidth float32, clr color.RGBA,
) ([]ebiten.Vertex, []uint16) {
	path.Reset()
	path.MoveTo(x0, y0)
	path.LineTo(x1, y1)
	strokeOp := &StrokeOptions{}
	strokeOp.Width = strokeWidth

	vs = vs[:0]
	is = is[:0]
	vs, is = path.AppendVerticesAndIndicesForStroke(vs, is, strokeOp)

	drawVerticesForUtil(dst, vs, is, clr)
	return vs, is
}

// DrawRect fills a rectangle with the specified width and color.
func DrawRect(
	path *Path, vs []ebiten.Vertex, is []uint16, dst *ebiten.Image,
	x, y, width, height float32, clr color.RGBA,
) ([]ebiten.Vertex, []uint16) {
	path.Reset()
	path.MoveTo(x, y)
	path.LineTo(x, y+height)
	path.LineTo(x+width, y+height)
	path.LineTo(x+width, y)

	vs = vs[:0]
	is = is[:0]
	vs, is = path.AppendVerticesAndIndicesForFilling(vs, is)

	drawVerticesForUtil(dst, vs, is, clr)
	return vs, is
}
