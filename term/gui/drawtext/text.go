// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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

package drawtext

import (
	"image"
	"image/color"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"

	"github.com/hajimehoshi/ebiten/v2"
)

// normalGlyphBlend and additiveGlyphBlend mirror the two glyph blend
// modes the renderer previously passed via DrawImageOptions: the
// default source-over used for ordinary text and the fully additive
// mode used for background runes (block/shade fills). Batches never mix
// blend modes because DrawTriangles issues one blend per call.
var (
	normalGlyphBlend = ebiten.Blend{
		BlendFactorSourceRGB:      ebiten.BlendFactorOne,
		BlendFactorDestinationRGB: ebiten.BlendFactorOneMinusSourceAlpha,
		BlendOperationRGB:         ebiten.BlendOperationAdd,
		BlendOperationAlpha:       ebiten.BlendOperationMax,
	}
	additiveGlyphBlend = ebiten.Blend{
		BlendFactorSourceRGB:        ebiten.BlendFactorOne,
		BlendFactorSourceAlpha:      ebiten.BlendFactorOne,
		BlendFactorDestinationRGB:   ebiten.BlendFactorOneMinusSourceAlpha,
		BlendFactorDestinationAlpha: ebiten.BlendFactorOne,
		BlendOperationRGB:           ebiten.BlendOperationAdd,
		BlendOperationAlpha:         ebiten.BlendOperationAdd,
	}
)

var normalGlyphOptions = ebiten.DrawTrianglesOptions{
	ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha,
	Blend:          normalGlyphBlend,
	AntiAlias:      false,
}

var additiveGlyphOptions = ebiten.DrawTrianglesOptions{
	ColorScaleMode: ebiten.ColorScaleModePremultipliedAlpha,
	Blend:          additiveGlyphBlend,
	AntiAlias:      false,
}

// Drawer batches glyph draws through an explicit shelf-packed atlas.
// Every glyph on an atlas page shares one source texture, so a run of
// glyphs that land on the same page and use the same blend mode issue as
// a single DrawTriangles. Draws are deferred and coalesced into
// run-length batches so submission order (and therefore overlap
// semantics for combining marks, ligatures and vertically offset cells)
// is preserved exactly; the renderer calls Flush after each glyph pass.
//
// A Drawer is not safe for concurrent use; the renderer drives it from
// the single GUI goroutine.
type Drawer struct {
	atlas *glyphAtlas

	// Current run being accumulated. runPage is -1 when no run is open.
	runPage     int
	runAdditive bool
	vertices    []ebiten.Vertex
	indices     []uint16
}

// New allocates storage for a new Drawer and initializes it.
func New() *Drawer {
	return &Drawer{
		atlas:   newGlyphAtlas(),
		runPage: -1,
	}
}

// Draw draws a given glyph on a given destination image dst. The draw is
// batched; the caller flushes with Flush.
func (d *Drawer) Draw(
	dst *ebiten.Image, ch rune, combining []rune,
	face font.Face, x, y int, clr color.RGBA,
) {
	cr, cg, cb, ca := clr.RGBA()
	col := [4]float32{
		float32(cr) / 0xffff,
		float32(cg) / 0xffff,
		float32(cb) / 0xffff,
		float32(ca) / 0xffff,
	}
	d.DrawGlyph(dst, ch, face, float64(x), float64(y), col, false)
}

// DrawGlyph accumulates one glyph quad for ch drawn with face at the
// destination pixel origin (dstX, dstY), tinted with the premultiplied
// color col. additive selects the additive background-rune blend mode.
// The glyph is packed into the atlas on first use and its quad is added
// to the current run-length batch for (page, blend); when the (page,
// blend) pair changes, the pending run is flushed to dst first so
// submission order is preserved.
//
// col holds premultiplied scale values in the same convention as
// ebiten's ColorScale (i.e. what DrawImage passes to its quad
// vertices), so batched output matches the previous per-glyph DrawImage
// path pixel for pixel.
func (d *Drawer) DrawGlyph(
	dst *ebiten.Image, ch rune, face font.Face,
	dstX, dstY float64, col [4]float32, additive bool,
) {
	g := d.atlas.get(face, ch)
	if g.empty {
		return
	}

	if d.runPage != g.page || d.runAdditive != additive {
		d.flushRun(dst)
		d.runPage = g.page
		d.runAdditive = additive
	}

	// Destination quad top-left mirrors the old drawGlyph translate:
	// options.GeoM.Translate(dstX, dstY) applied to the glyph's mask
	// origin (b.Min - offset).
	topX := dstX + fixed26_6ToFloat64(g.bounds.Min.X-g.offset.X)
	topY := dstY + fixed26_6ToFloat64(g.bounds.Min.Y-g.offset.Y)
	w := float32(g.rect.Dx())
	h := float32(g.rect.Dy())
	dx0 := float32(topX)
	dy0 := float32(topY)
	dx1 := dx0 + w
	dy1 := dy0 + h
	sx0 := float32(g.rect.Min.X)
	sy0 := float32(g.rect.Min.Y)
	sx1 := float32(g.rect.Max.X)
	sy1 := float32(g.rect.Max.Y)

	base := uint16(len(d.vertices))
	d.vertices = append(d.vertices,
		ebiten.Vertex{DstX: dx0, DstY: dy0, SrcX: sx0, SrcY: sy0,
			ColorR: col[0], ColorG: col[1], ColorB: col[2], ColorA: col[3]},
		ebiten.Vertex{DstX: dx1, DstY: dy0, SrcX: sx1, SrcY: sy0,
			ColorR: col[0], ColorG: col[1], ColorB: col[2], ColorA: col[3]},
		ebiten.Vertex{DstX: dx0, DstY: dy1, SrcX: sx0, SrcY: sy1,
			ColorR: col[0], ColorG: col[1], ColorB: col[2], ColorA: col[3]},
		ebiten.Vertex{DstX: dx1, DstY: dy1, SrcX: sx1, SrcY: sy1,
			ColorR: col[0], ColorG: col[1], ColorB: col[2], ColorA: col[3]},
	)
	d.indices = append(d.indices,
		base, base+1, base+2, base+1, base+2, base+3)
}

// Flush issues any pending glyph batch to dst and closes the current
// run. The renderer calls it after each glyph pass and after the cursor
// glyph, so batched glyphs land in submission order relative to the
// solid-geometry passes around them.
func (d *Drawer) Flush(dst *ebiten.Image) {
	d.flushRun(dst)
}

func (d *Drawer) flushRun(dst *ebiten.Image) {
	if len(d.indices) == 0 {
		d.runPage = -1
		return
	}
	opts := &normalGlyphOptions
	if d.runAdditive {
		opts = &additiveGlyphOptions
	}
	dst.DrawTriangles(d.vertices, d.indices, d.atlas.page(d.runPage), opts)
	d.vertices = d.vertices[:0]
	d.indices = d.indices[:0]
	d.runPage = -1
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

func fixed26_6ToFloat64(x fixed.Int26_6) float64 {
	return float64(x>>6) + float64(x&((1<<6)-1))/float64(1<<6)
}

type glyphCacheKey struct {
	face font.Face
	r    rune
}
