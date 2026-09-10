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

	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// atlasPageSize is the width and height in pixels of every glyph atlas
// page. It is large enough to hold thousands of cell-sized glyph masks
// per page so a normal working set fits in one or two pages, and each
// page becomes a single DrawTriangles source.
const atlasPageSize = 1024

// atlasPadding separates packed glyphs by one transparent pixel so
// bilinear sampling at a quad edge cannot bleed a neighbour's coverage.
// Sampling is nearest here, but the padding keeps the packing robust if
// that ever changes.
const atlasPadding = 1

// atlasGlyph records where a rasterized glyph mask lives inside the
// atlas and the geometry needed to place its quad. page indexes into
// glyphAtlas.pages; rect is the mask's pixel region within that page;
// bounds/offset mirror the font glyph metrics captured at rasterization
// time so the drawing code positions the quad exactly like the previous
// per-glyph DrawImage path did.
type atlasGlyph struct {
	page   int
	rect   image.Rectangle
	bounds fixed.Rectangle26_6
	offset fixed.Point26_6
	// empty marks a glyph with no pixels (space, zero-size bounds); it
	// is cached so repeated lookups stay cheap but contributes no quad.
	empty bool
}

// glyphAtlas shelf-packs rasterized white-mask glyphs into fixed-size
// ebiten image pages. All glyphs on a page share one source texture, so
// their quads batch into a single DrawTriangles regardless of how the
// underlying ebiten internal atlas would have fragmented per-glyph
// images.
//
// A glyphAtlas is not safe for concurrent use; the renderer drives it
// from the single GUI goroutine.
type glyphAtlas struct {
	pages []*ebiten.Image
	cache map[glyphCacheKey]atlasGlyph

	// Shelf-packing cursor into the current (last) page.
	shelfX      int
	shelfY      int
	shelfHeight int

	// Scratch RGBA pixel buffer reused to rasterize a glyph mask before
	// uploading it to a page, so steady-state rendering does not
	// allocate per glyph. Kept as a flat byte slice so it can back a
	// tightly packed (Stride == 4*w) RGBA of the exact glyph size.
	scratch []byte
}

func newGlyphAtlas() *glyphAtlas {
	return &glyphAtlas{cache: make(map[glyphCacheKey]atlasGlyph, 1024)}
}

// get returns the placement for (face, r), rasterizing and packing it on
// first use. The returned atlasGlyph is valid until the next reset.
func (a *glyphAtlas) get(face font.Face, r rune) atlasGlyph {
	key := glyphCacheKey{face: face, r: r}
	if g, ok := a.cache[key]; ok {
		return g
	}
	g := a.rasterizeAndPack(face, r)
	a.cache[key] = g
	return g
}

// rasterizeAndPack renders a single glyph into a white coverage mask and
// packs it into an atlas page. The mask geometry matches the previous
// getGlyphImage path exactly so drawn positions are unchanged.
func (a *glyphAtlas) rasterizeAndPack(face font.Face, r rune) atlasGlyph {
	b, _, _ := face.GlyphBounds(r)
	offset := fixed.Point26_6{
		X: b.Min.X & ((1 << 6) - 1),
		Y: b.Min.Y & ((1 << 6) - 1),
	}
	w, h := (b.Max.X - b.Min.X).Ceil(), (b.Max.Y - b.Min.Y).Ceil()
	if w == 0 || h == 0 {
		return atlasGlyph{empty: true, bounds: b, offset: offset}
	}
	if b.Min.X&((1<<6)-1) != 0 {
		w++
	}
	if b.Min.Y&((1<<6)-1) != 0 {
		h++
	}

	pix := a.rasterizeMask(face, r, b, offset, w, h)

	rect := a.alloc(w, h)
	page := a.pages[len(a.pages)-1]
	page.SubImage(rect).(*ebiten.Image).WritePixels(pix)

	return atlasGlyph{
		page:   len(a.pages) - 1,
		rect:   rect,
		bounds: b,
		offset: offset,
	}
}

// rasterizeMask renders the glyph coverage into a tightly packed w×h
// RGBA (Stride == 4*w) backed by the reused scratch buffer and returns
// the packed bytes. Tight packing is required: WritePixels uploads the
// slice as a contiguous 4*w*h run, so a wider backing stride (e.g. from
// a SubImage of an over-sized scratch) would be misread row by row and
// scatter the glyph mask. The returned slice aliases the scratch and is
// only valid until the next call.
func (a *glyphAtlas) rasterizeMask(
	face font.Face, r rune, b fixed.Rectangle26_6, offset fixed.Point26_6, w, h int,
) []byte {
	need := 4 * w * h
	if cap(a.scratch) < need {
		a.scratch = make([]byte, need)
	}
	pix := a.scratch[:need]
	clear(pix)
	dst := &image.RGBA{Pix: pix, Stride: 4 * w, Rect: image.Rect(0, 0, w, h)}

	fd := font.Drawer{Dst: dst, Src: image.White, Face: face}
	fd.Dot = fixed.Point26_6{X: -b.Min.X + offset.X, Y: -b.Min.Y + offset.Y}
	fd.DrawString(string(r))
	return pix
}

// alloc reserves a w×h region on the current page using a simple shelf
// (next-fit) packer, opening a new page when the glyph does not fit.
func (a *glyphAtlas) alloc(w, h int) image.Rectangle {
	if len(a.pages) == 0 {
		a.newPage()
	}
	// Advance to a new shelf if the glyph does not fit on the current row.
	if a.shelfX+w+atlasPadding > atlasPageSize {
		a.shelfX = 0
		a.shelfY += a.shelfHeight + atlasPadding
		a.shelfHeight = 0
	}
	// Open a new page if the glyph does not fit vertically.
	if a.shelfY+h+atlasPadding > atlasPageSize {
		a.newPage()
	}
	rect := image.Rect(a.shelfX, a.shelfY, a.shelfX+w, a.shelfY+h)
	a.shelfX += w + atlasPadding
	if h > a.shelfHeight {
		a.shelfHeight = h
	}
	return rect
}

func (a *glyphAtlas) newPage() {
	a.pages = append(a.pages, ebiten.NewImage(atlasPageSize, atlasPageSize))
	a.shelfX = 0
	a.shelfY = 0
	a.shelfHeight = 0
}

// page returns the ebiten image backing the given page index.
func (a *glyphAtlas) page(i int) *ebiten.Image {
	return a.pages[i]
}

// pageCount reports how many pages the atlas currently holds.
func (a *glyphAtlas) pageCount() int {
	return len(a.pages)
}

func (a *glyphAtlas) deallocate() {
	for _, page := range a.pages {
		page.Deallocate()
	}
	a.pages = nil
	a.cache = nil
	a.scratch = nil
}
