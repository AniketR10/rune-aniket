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

// Draw draws a given text on a given destination image dst.
func Draw(
	dst *ebiten.Image, text string,
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
	DrawWithOptions(dst, text, face, &op)
}

// DrawWithOptions draws a given text on a given destination image dst.
func DrawWithOptions(
	dst *ebiten.Image, text string,
	face font.Face, options *ebiten.DrawImageOptions,
) {
	fc := faceWithCacheFromFace(face)

	var dx, dy fixed.Int26_6
	prevR := rune(-1)

	faceHeight := fc.Metrics().Height

	for _, r := range text {
		if prevR >= 0 {
			dx += fc.Kern(prevR, r)
		}
		if r == '\n' {
			dx = 0
			dy += faceHeight
			prevR = rune(-1)
			continue
		}

		// Adjust the position to the integers.
		// The current glyph images assume that they are rendered on integer positions so far.
		b, a, _ := fc.GlyphBounds(r)
		offset := fixed.Point26_6{
			X: (adjustOffsetGranularity(dx) + b.Min.X) & ((1 << 6) - 1),
			Y: b.Min.Y & ((1 << 6) - 1),
		}
		img := getGlyphImage(fc, r, offset)
		drawGlyph(dst, img, fixed.Point26_6{
			X: dx + b.Min.X - offset.X,
			Y: dy + b.Min.Y - offset.Y,
		}, options)
		dx += a

		prevR = r
	}
}

// BoundString returns the measured size of a given string using a given font.
// This method will return the exact size in pixels that a string drawn by Draw will be.
// The bound's origin point indicates the origin position in this figure:
// https://developer.apple.com/library/archive/documentation/TextFonts/Conceptual/CocoaTextArchitecture/Art/glyphterms_2x.png.
func BoundString(face font.Face, text string) image.Rectangle {

	fc := faceWithCacheFromFace(face)

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

// CacheGlyphs precaches the glyphs for the given text and the given font face into the cache.
func CacheGlyphs(face font.Face, text string) {
	fc := faceWithCacheFromFace(face)

	var dx fixed.Int26_6
	prevR := rune(-1)
	for _, r := range text {
		if prevR >= 0 {
			dx += fc.Kern(prevR, r)
		}
		if r == '\n' {
			dx = 0
			continue
		}
		b, a, _ := fc.GlyphBounds(r)

		// Cache all 4 variations for one rune (#2528).
		for i := 0; i < 4; i++ {
			offset := fixed.Point26_6{
				X: (fixed.Int26_6(i*(1<<4)) + b.Min.X) & ((1 << 6) - 1),
				Y: b.Min.Y & ((1 << 6) - 1),
			}
			getGlyphImage(fc, r, offset)
		}

		dx += a
		prevR = r
	}
}

// FaceWithLineHeight returns a font.Face with the given lineHeight in pixels.
// The returned face will otherwise have the same glyphs and metrics as face.
func FaceWithLineHeight(face font.Face, lineHeight float64) font.Face {
	return faceWithLineHeight{
		face:       face,
		lineHeight: fixed.Int26_6(lineHeight * (1 << 6)),
	}
}

type faceWithLineHeight struct {
	face       font.Face
	lineHeight fixed.Int26_6
}

func (f faceWithLineHeight) Close() error {
	return f.face.Close()
}

func (f faceWithLineHeight) Glyph(origin fixed.Point26_6, r rune) (dr image.Rectangle, mask image.Image, maskp image.Point, advance fixed.Int26_6, ok bool) {
	return f.face.Glyph(origin, r)
}

func (f faceWithLineHeight) GlyphBounds(r rune) (bounds fixed.Rectangle26_6, advance fixed.Int26_6, ok bool) {
	return f.face.GlyphBounds(r)
}

func (f faceWithLineHeight) GlyphAdvance(r rune) (advance fixed.Int26_6, ok bool) {
	return f.face.GlyphAdvance(r)
}

func (f faceWithLineHeight) Kern(r0, r1 rune) fixed.Int26_6 {
	return f.face.Kern(r0, r1)
}

func (f faceWithLineHeight) Metrics() font.Metrics {
	m := f.face.Metrics()
	m.Height = f.lineHeight
	return m
}


func fixed26_6ToFloat64(x fixed.Int26_6) float64 {
	return float64(x>>6) + float64(x&((1<<6)-1))/float64(1<<6)
}

func adjustOffsetGranularity(x fixed.Int26_6) fixed.Int26_6 {
	return x / (1 << 4) * (1 << 4)
}

func drawGlyph(dst *ebiten.Image, img *ebiten.Image, topleft fixed.Point26_6, op *ebiten.DrawImageOptions) {
	if img == nil {
		return
	}

	op2 := &ebiten.DrawImageOptions{}
	if op != nil {
		*op2 = *op
		op2.GeoM.Reset()
	}

	op2.GeoM.Translate(fixed26_6ToFloat64(topleft.X), fixed26_6ToFloat64(topleft.Y))
	if op != nil {
		op2.GeoM.Concat(op.GeoM)
	}

	dst.DrawImage(img, op2)
}

type glyphImageCacheKey struct {
	rune    rune
	xoffset fixed.Int26_6
}

type glyphImageCacheEntry struct {
	image *ebiten.Image
}

var (
	glyphImageCache = map[*faceWithCache]map[glyphImageCacheKey]*glyphImageCacheEntry{}
)

func getGlyphImage(face *faceWithCache, r rune, offset fixed.Point26_6) *ebiten.Image {
	if _, ok := glyphImageCache[face]; !ok {
		glyphImageCache[face] = map[glyphImageCacheKey]*glyphImageCacheEntry{}
	}

	key := glyphImageCacheKey{
		rune:    r,
		xoffset: offset.X,
	}
	if e, ok := glyphImageCache[face][key]; ok {
		return e.image
	}

	b, _, _ := face.GlyphBounds(r)
	w, h := (b.Max.X - b.Min.X).Ceil(), (b.Max.Y - b.Min.Y).Ceil()
	if w == 0 || h == 0 {
		glyphImageCache[face][key] = &glyphImageCacheEntry{
			image: nil,
		}
		return nil
	}

	if b.Min.X&((1<<6)-1) != 0 {
		w++
	}
	if b.Min.Y&((1<<6)-1) != 0 {
		h++
	}
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))

	d := font.Drawer{
		Dst:  rgba,
		Src:  image.White,
		Face: face,
	}

	x, y := -b.Min.X, -b.Min.Y
	x += offset.X
	y += offset.Y
	d.Dot = fixed.Point26_6{X: x, Y: y}
	d.DrawString(string(r))

	img := ebiten.NewImageFromImage(rgba)
	glyphImageCache[face][key] = &glyphImageCacheEntry{
		image: img,
	}

	return img
}
