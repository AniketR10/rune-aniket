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
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

const cacheInitialCapacity = 1024

var _ font.Face = (*cacheFace)(nil)

type cacheFace struct {
	f font.Face

	metrics           font.Metrics
	glyphCache        map[glyphCacheKey]glyphCacheValue
	glyphBoundsCache  map[rune]glyphBoundsCacheValue
	glyphAdvanceCache map[rune]glyphAdvanceCacheValue
	kernCache         map[kernCacheKey]fixed.Int26_6
}

func newCacheFace(f font.Face) *cacheFace {
	return &cacheFace{
		f:                 f,
		metrics:           f.Metrics(),
		glyphCache:        make(map[glyphCacheKey]glyphCacheValue, cacheInitialCapacity),
		glyphBoundsCache:  make(map[rune]glyphBoundsCacheValue, cacheInitialCapacity),
		glyphAdvanceCache: make(map[rune]glyphAdvanceCacheValue, cacheInitialCapacity),
		kernCache:         make(map[kernCacheKey]fixed.Int26_6, cacheInitialCapacity),
	}
}

func (f *cacheFace) Glyph(dot fixed.Point26_6, r rune) (
	dr image.Rectangle, mask image.Image,
	maskp image.Point, advance fixed.Int26_6, ok bool,
) {
	key := glyphCacheKey{dot: dot, r: r}
	if v, ok := f.glyphCache[key]; ok {
		return v.dr, v.mask, v.maskp, v.advance, v.ok
	}

	dr, mask, maskp, advance, ok = f.f.Glyph(dot, r)
	mask = copyImage(mask)

	f.glyphCache[key] = glyphCacheValue{
		dr:      dr,
		mask:    mask,
		maskp:   maskp,
		advance: advance,
		ok:      ok,
	}
	return
}

func copyImage(src image.Image) image.Image {
	bounds := src.Bounds()
	dest := image.NewRGBA(bounds)
	draw.Draw(dest, bounds, src, bounds.Min, draw.Src)
	return dest
}

func (f *cacheFace) GlyphBounds(r rune) (
	bounds fixed.Rectangle26_6,
	advance fixed.Int26_6, ok bool,
) {
	if v, ok := f.glyphBoundsCache[r]; ok {
		return v.bounds, v.advance, v.ok
	}

	bounds, advance, ok = f.f.GlyphBounds(r)
	f.glyphBoundsCache[r] = glyphBoundsCacheValue{
		bounds:  bounds,
		advance: advance,
		ok:      ok,
	}
	return
}

func (f *cacheFace) GlyphAdvance(r rune) (advance fixed.Int26_6, ok bool) {
	if v, ok := f.glyphAdvanceCache[r]; ok {
		return v.advance, v.ok
	}

	advance, ok = f.f.GlyphAdvance(r)
	f.glyphAdvanceCache[r] = glyphAdvanceCacheValue{
		advance: advance,
		ok:      ok,
	}
	return
}

func (f *cacheFace) Kern(r0, r1 rune) fixed.Int26_6 {
	key := kernCacheKey{r0: r0, r1: r1}
	if v, ok := f.kernCache[key]; ok {
		return v
	}

	v := f.f.Kern(r0, r1)
	f.kernCache[key] = v
	return v
}

func (f *cacheFace) Metrics() font.Metrics {
	return f.metrics
}

func (f *cacheFace) Close() error {
	return f.f.Close()
}

type glyphBoundsCacheValue struct {
	bounds  fixed.Rectangle26_6
	advance fixed.Int26_6
	ok      bool
}

type glyphAdvanceCacheValue struct {
	advance fixed.Int26_6
	ok      bool
}

type kernCacheKey struct {
	r0 rune
	r1 rune
}

type glyphCacheKey struct {
	dot fixed.Point26_6
	r   rune
}
type glyphCacheValue struct {
	dr      image.Rectangle
	mask    image.Image
	maskp   image.Point
	advance fixed.Int26_6
	ok      bool
}
