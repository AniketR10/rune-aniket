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

package drawtext

import (
	"image"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

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

type faceWithCache struct {
	f       font.Face
	metrics font.Metrics

	glyphBoundsCache  map[rune]glyphBoundsCacheValue
	glyphAdvanceCache map[rune]glyphAdvanceCacheValue
	kernCache         map[kernCacheKey]fixed.Int26_6
}

func (f *faceWithCache) Close() error {
	if err := f.f.Close(); err != nil {
		return err
	}

	f.glyphBoundsCache = nil
	f.glyphAdvanceCache = nil
	f.kernCache = nil
	return nil
}

var faceWithCacheCache map[font.Face]*faceWithCache

func faceWithCacheFromFace(face font.Face) *faceWithCache {
	if f, ok := faceWithCacheCache[face]; ok {
		return f
	}

	f := &faceWithCache{
		f: face,
	}
	if faceWithCacheCache == nil {
		faceWithCacheCache = map[font.Face]*faceWithCache{}
	}
	faceWithCacheCache[face] = f
	return f
}

func (f *faceWithCache) Glyph(dot fixed.Point26_6, r rune) (dr image.Rectangle, mask image.Image, maskp image.Point, advance fixed.Int26_6, ok bool) {
	// TODO: Move glyphImageCache to here.
	return f.f.Glyph(dot, r)
}

func (f *faceWithCache) GlyphBounds(r rune) (bounds fixed.Rectangle26_6, advance fixed.Int26_6, ok bool) {
	if v, ok := f.glyphBoundsCache[r]; ok {
		return v.bounds, v.advance, v.ok
	}

	bounds, advance, ok = f.f.GlyphBounds(r)
	if f.glyphBoundsCache == nil {
		f.glyphBoundsCache = map[rune]glyphBoundsCacheValue{}
	}
	f.glyphBoundsCache[r] = glyphBoundsCacheValue{
		bounds:  bounds,
		advance: advance,
		ok:      ok,
	}
	return
}

func (f *faceWithCache) GlyphAdvance(r rune) (advance fixed.Int26_6, ok bool) {
	if v, ok := f.glyphAdvanceCache[r]; ok {
		return v.advance, v.ok
	}

	advance, ok = f.f.GlyphAdvance(r)
	if f.glyphAdvanceCache == nil {
		f.glyphAdvanceCache = map[rune]glyphAdvanceCacheValue{}
	}
	f.glyphAdvanceCache[r] = glyphAdvanceCacheValue{
		advance: advance,
		ok:      ok,
	}
	return
}

func (f *faceWithCache) Kern(r0, r1 rune) fixed.Int26_6 {
	key := kernCacheKey{r0: r0, r1: r1}
	if v, ok := f.kernCache[key]; ok {
		return v
	}

	v := f.f.Kern(r0, r1)
	if f.kernCache == nil {
		f.kernCache = map[kernCacheKey]fixed.Int26_6{}
	}
	f.kernCache[key] = v
	return v
}

func (f *faceWithCache) Metrics() font.Metrics {
	if f.metrics != (font.Metrics{}) {
		return f.metrics
	}
	f.metrics = f.f.Metrics()
	return f.metrics
}
