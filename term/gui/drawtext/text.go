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

// Package text offers functions to draw texts on an Ebitengine's image.
//
// For the example using a TTF font, see font package in the examples.
package drawtext

import (
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/hajimehoshi/ebiten/v2"
)

// Drawer provides helpers to draw text glyphs using ebiten engine.
type Drawer struct {
	glyphCache map[glyphCacheKey]glyphCacheValue
}

// New allocates storage for a new Drawer and initializes it.
func New() *Drawer {
	return &Drawer{
		glyphCache: make(map[glyphCacheKey]glyphCacheValue, 1024),
	}
}

// Draw draws a given text on a given destination image dst.
func (d *Drawer) Draw(
	dst *ebiten.Image, ch rune, combining []rune,
	face font.Face, x, y int, clr color.RGBA,
) {
	var op ebiten.DrawImageOptions
	op.GeoM.Translate(float64(x), float64(y))
	cr, cg, cb, ca := clr.RGBA()
	op.ColorScale.Scale(
		float32(cr)/0xffff,
		float32(cg)/0xffff,
		float32(cb)/0xffff,
		float32(ca)/0xffff,
	)
	d.DrawWithOptions(dst, ch, combining, face, &op)
}

// DrawWithOptions draws a given text on a given destination image dst.
func (d *Drawer) DrawWithOptions(
	dst *ebiten.Image, ch rune, combining []rune,
	fc font.Face, options *ebiten.DrawImageOptions,
) {
	offset, b, img := d.getGlyphImage(fc, ch)
	d.drawGlyph(dst, img, fixed.Point26_6{
		X: b.Min.X - offset.X,
		Y: b.Min.Y - offset.Y,
	}, options)
}

// BoundString returns the measured size of a given string using a given font.
// This method will return the exact size in pixels that a string drawn by Draw will be.
// The bound's origin point indicates the origin position in this figure:
// https://developer.apple.com/library/archive/documentation/TextFonts/Conceptual/CocoaTextArchitecture/Art/glyphterms_2x.png.
func BoundString(fc font.Face, text string) image.Rectangle {
	m := fc.Metrics()
	faceHeight := m.Height

	fx, fy := fixed.I(0), fixed.I(0)
	prevR := rune(-1)

	var bounds fixed.Rectangle26_6
	for _, r := range text {
		if prevR >= 0 {
			fx += fc.Kern(prevR, r)
		}
		if r == '\n' {
			fx = fixed.I(0)
			fy += faceHeight
			prevR = rune(-1)
			continue
		}

		b, a, _ := fc.GlyphBounds(r)
		b.Min.X += fx
		b.Max.X += fx
		b.Min.Y += fy
		b.Max.Y += fy
		bounds = bounds.Union(b)

		fx += a
		prevR = r
	}

	return image.Rect(
		bounds.Min.X.Floor(),
		bounds.Min.Y.Floor(),
		bounds.Max.X.Ceil(),
		bounds.Max.Y.Ceil(),
	)
}

func (d *Drawer) drawGlyph(
	dst *ebiten.Image, img *ebiten.Image,
	topleft fixed.Point26_6, op *ebiten.DrawImageOptions,
) {
	if img == nil {
		return
	}

	var op2 ebiten.DrawImageOptions
	if op != nil {
		op2 = *op
		op2.GeoM.Reset()
	}

	op2.GeoM.Translate(fixed26_6ToFloat64(topleft.X), fixed26_6ToFloat64(topleft.Y))
	if op != nil {
		op2.GeoM.Concat(op.GeoM)
	}

	dst.DrawImage(img, &op2)
}

func (d *Drawer) getGlyphImage(face font.Face, r rune) (
	offset fixed.Point26_6,
	b fixed.Rectangle26_6,
	img *ebiten.Image,
) {
	cacheKey := glyphCacheKey{face: face, r: r}
	if v, ok := d.glyphCache[cacheKey]; ok {
		return v.offset, v.b, v.img
	}
	b, _, _ = face.GlyphBounds(r)
	offset = fixed.Point26_6{
		X: b.Min.X & ((1 << 6) - 1),
		Y: b.Min.Y & ((1 << 6) - 1),
	}
	w, h := (b.Max.X - b.Min.X).Ceil(), (b.Max.Y - b.Min.Y).Ceil()
	if w == 0 || h == 0 {
		return
	}

	if b.Min.X&((1<<6)-1) != 0 {
		w++
	}
	if b.Min.Y&((1<<6)-1) != 0 {
		h++
	}
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))

	fd := font.Drawer{
		Dst:  rgba,
		Src:  image.White,
		Face: face,
	}

	x, y := -b.Min.X, -b.Min.Y
	x += offset.X
	y += offset.Y
	fd.Dot = fixed.Point26_6{X: x, Y: y}
	fd.DrawString(string(r))

	img = ebiten.NewImageFromImage(rgba)
	d.glyphCache[cacheKey] = glyphCacheValue{
		img:    img,
		b:      b,
		offset: offset,
	}
	return
}

func fixed26_6ToFloat64(x fixed.Int26_6) float64 {
	return float64(x>>6) + float64(x&((1<<6)-1))/float64(1<<6)
}

type glyphCacheKey struct {
	face font.Face
	r    rune
}

type glyphCacheValue struct {
	offset fixed.Point26_6
	b      fixed.Rectangle26_6
	img    *ebiten.Image
}
