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

package font

import (
	"image"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

var _ font.Face = (*mockFace)(nil)

type mockFace struct {
	glyph   int
	bounds  int
	close   int
	advance int
	kern    int
	metrics int
}

func newMockFace() *mockFace {
	return &mockFace{}
}

func (m *mockFace) Close() (ret error) {
	m.close++
	return
}

func (m *mockFace) Glyph(dot fixed.Point26_6, r rune) (
	dr image.Rectangle, mask image.Image,
	maskp image.Point, advance fixed.Int26_6, ok bool,
) {
	m.glyph++
	mask = image.NewRGBA(image.Rectangle{})
	return
}

func (m *mockFace) GlyphBounds(r rune) (
	bounds fixed.Rectangle26_6, advance fixed.Int26_6, ok bool,
) {
	m.bounds++
	return
}

func (m *mockFace) GlyphAdvance(r rune) (
	advance fixed.Int26_6, ok bool,
) {
	m.advance++
	return
}

func (m *mockFace) Kern(r0, r1 rune) fixed.Int26_6 {
	m.kern++
	return fixed.I(0)
}

func (m *mockFace) Metrics() font.Metrics {
	m.metrics++
	return font.Metrics{}
}
