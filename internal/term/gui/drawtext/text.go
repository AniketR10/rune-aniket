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
		BlendOperationAlpha:       ebiten.BlendOperationAdd,
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
	atlas      *glyphAtlas
	colorAtlas *colorAtlas

	// Current run being accumulated. runPage is -1 when no run is open.
	// runColor selects the color atlas over the mask atlas; the two never
	// share a DrawTriangles because they draw from different textures.
	runPage     int
	runColor    bool
	runAdditive bool
	vertices    []ebiten.Vertex
	indices     []uint16
}

// New allocates storage for a new Drawer and initializes it.
func New() *Drawer {
	return &Drawer{
		atlas:      newGlyphAtlas(),
		colorAtlas: newColorAtlas(),
		runPage:    -1,
	}
}

// Deallocate releases the graphics storage owned by this drawer.
func (d *Drawer) Deallocate() {
	if d.atlas != nil {
		d.atlas.deallocate()
		d.atlas = nil
	}
	if d.colorAtlas != nil {
		d.colorAtlas.deallocate()
		d.colorAtlas = nil
	}
	d.vertices = nil
	d.indices = nil
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
//
// col holds premultiplied scale values in the same convention as
// ebiten's ColorScale.
func (d *Drawer) DrawGlyph(
	dst *ebiten.Image, ch rune, face font.Face,
	dstX, dstY float64, col [4]float32, additive bool,
) {
	g := d.atlas.get(face, ch)
	if g.empty {
		return
	}

	if d.runColor || d.runPage != g.page || d.runAdditive != additive {
		d.flushRun(dst)
		d.runColor = false
		d.runPage = g.page
		d.runAdditive = additive
	}

	// Destination quad top-left mirrors the old drawGlyph translate:
	// options.GeoM.Translate(dstX, dstY) applied to the glyph's mask
	// origin (b.Min - offset).
	topX := dstX + fixed26_6ToFloat64(g.bounds.Min.X-g.offset.X)
	topY := dstY + fixed26_6ToFloat64(g.bounds.Min.Y-g.offset.Y)
	d.appendQuad(float32(topX), float32(topY), g.rect, col)
}

// DrawColorGlyph accumulates one color emoji quad for cluster, rasterized
// through src into the cell box at (dstX, dstY) sized cellW×cellH. The
// glyph draws untinted so its own multi-color bitmap survives. A change of
// source page, or a switch to or from the mask path, flushes the pending
// run first to preserve submission order.
func (d *Drawer) DrawColorGlyph(
	dst *ebiten.Image, cluster []rune, src ColorGlyphSource,
	dstX, dstY float64, cellW, cellH int,
) {
	g := d.colorAtlas.get(src, cluster, cellW, cellH)
	if g.empty {
		return
	}

	if !d.runColor || d.runPage != g.page {
		d.flushRun(dst)
		d.runColor = true
		d.runAdditive = false
		d.runPage = g.page
	}

	// Identity color leaves the bitmap's own premultiplied pixels
	// untouched under ColorScaleModePremultipliedAlpha.
	identity := [4]float32{1, 1, 1, 1}
	topX := float32(dstX) + float32(g.offX)
	topY := float32(dstY) + float32(g.offY)
	d.appendQuad(topX, topY, g.rect, identity)
}

// appendQuad appends a textured quad at (dstX, dstY) sampling atlas region
// rect, tinted per-vertex by col. Both glyph paths funnel through it so
// vertex layout and winding stay identical.
func (d *Drawer) appendQuad(
	dstX, dstY float32, rect image.Rectangle, col [4]float32,
) {
	w := float32(rect.Dx())
	h := float32(rect.Dy())
	dx0, dy0 := dstX, dstY
	dx1, dy1 := dx0+w, dy0+h
	sx0 := float32(rect.Min.X)
	sy0 := float32(rect.Min.Y)
	sx1 := float32(rect.Max.X)
	sy1 := float32(rect.Max.Y)

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
	var src *ebiten.Image
	if d.runColor {
		src = d.colorAtlas.page(d.runPage)
	} else {
		src = d.atlas.page(d.runPage)
	}
	dst.DrawTriangles(d.vertices, d.indices, src, opts)
	d.vertices = d.vertices[:0]
	d.indices = d.indices[:0]
	d.runPage = -1
	d.runColor = false
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
