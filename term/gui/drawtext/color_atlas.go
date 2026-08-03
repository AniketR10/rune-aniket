// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

	"github.com/hajimehoshi/ebiten/v2"
)

// ColorGlyphSource rasterizes a grapheme cluster to a premultiplied RGBA
// sized to fit a cellW×cellH box, or reports absence. It is the color
// path's only dependency on the emoji font stack; *emoji.Face satisfies it.
type ColorGlyphSource interface {
	Glyph(cluster []rune, cellW, cellH int) (*image.RGBA, bool)
}

// colorGlyphKey identifies a packed color glyph. The cell box is part of
// the key because color glyphs are rasterized at a concrete pixel size,
// so a size change must not reuse a stale-size bitmap.
type colorGlyphKey struct {
	cluster      string
	cellW, cellH int
}

// colorGlyph records where a decoded color bitmap lives in the atlas and
// the offset that centers it within the cell box.
type colorGlyph struct {
	page       int
	rect       image.Rectangle
	offX, offY int
	// empty caches a cluster with no color bitmap so lookups stay cheap.
	empty bool
}

// colorAtlas shelf-packs premultiplied RGBA emoji bitmaps into fixed-size
// ebiten pages so a run batches into one DrawTriangles. It mirrors
// glyphAtlas but stores color instead of a coverage mask, and is not safe
// for concurrent use.
type colorAtlas struct {
	pages []*ebiten.Image
	cache map[colorGlyphKey]colorGlyph

	shelfX      int
	shelfY      int
	shelfHeight int
}

func newColorAtlas() *colorAtlas {
	return &colorAtlas{cache: make(map[colorGlyphKey]colorGlyph, 256)}
}

// get returns the placement for (cluster, cellW, cellH), rasterizing via
// src and packing it on first use. The returned colorGlyph is valid
// until the next reset.
func (a *colorAtlas) get(src ColorGlyphSource, cluster []rune, cellW, cellH int) colorGlyph {
	key := colorGlyphKey{cluster: string(cluster), cellW: cellW, cellH: cellH}
	if g, ok := a.cache[key]; ok {
		return g
	}
	g := a.rasterizeAndPack(src, cluster, cellW, cellH)
	a.cache[key] = g
	return g
}

// rasterizeAndPack decodes the cluster's color bitmap through src and
// uploads it into a page. The bitmap is centered in the cell box; the
// residual margins become the draw offsets.
func (a *colorAtlas) rasterizeAndPack(
	src ColorGlyphSource, cluster []rune, cellW, cellH int,
) colorGlyph {
	img, ok := src.Glyph(cluster, cellW, cellH)
	if !ok || img == nil {
		return colorGlyph{empty: true}
	}
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w <= 0 || h <= 0 {
		return colorGlyph{empty: true}
	}

	rect := a.alloc(w, h)
	page := a.pages[len(a.pages)-1]
	page.SubImage(rect).(*ebiten.Image).WritePixels(img.Pix)

	return colorGlyph{
		page: len(a.pages) - 1,
		rect: rect,
		offX: (cellW - w) / 2,
		offY: (cellH - h) / 2,
	}
}

// alloc reserves a w×h region on the current page using the same shelf
// (next-fit) packer as glyphAtlas, opening a new page when the glyph
// does not fit.
func (a *colorAtlas) alloc(w, h int) image.Rectangle {
	if len(a.pages) == 0 {
		a.newPage()
	}
	if a.shelfX+w+atlasPadding > atlasPageSize {
		a.shelfX = 0
		a.shelfY += a.shelfHeight + atlasPadding
		a.shelfHeight = 0
	}
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

func (a *colorAtlas) newPage() {
	a.pages = append(a.pages, ebiten.NewImage(atlasPageSize, atlasPageSize))
	a.shelfX = 0
	a.shelfY = 0
	a.shelfHeight = 0
}

// page returns the ebiten image backing the given page index.
func (a *colorAtlas) page(i int) *ebiten.Image {
	return a.pages[i]
}

// pageCount reports how many pages the atlas currently holds.
func (a *colorAtlas) pageCount() int {
	return len(a.pages)
}
