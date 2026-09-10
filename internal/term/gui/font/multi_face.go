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

	"github.com/ernestrc/go-multierror"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

var _ font.Face = (*multi)(nil)

type multi struct {
	faces     []font.Face
	preferred int
}

func newMultiFace(preferred int, faces ...font.Face) *multi {
	if len(faces) == 0 {
		panic("multi face with no faces")
	}
	if preferred < 0 || preferred >= len(faces) {
		panic("preferred must be an index with the preferred font for calculations")
	}
	return &multi{faces: faces, preferred: preferred}
}

func (m *multi) Close() (ret error) {
	for _, face := range m.faces {
		if err := face.Close(); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return
}

func (m *multi) Glyph(dot fixed.Point26_6, r rune) (
	dr image.Rectangle, mask image.Image,
	maskp image.Point, advance fixed.Int26_6, ok bool,
) {
	for _, face := range m.faces {
		dr, mask, maskp, advance, ok = face.Glyph(dot, r)
		if ok {
			return
		}
	}
	return m.faces[m.preferred].Glyph(dot, r)
}

func (m *multi) GlyphBounds(r rune) (
	bounds fixed.Rectangle26_6, advance fixed.Int26_6, ok bool,
) {
	for _, face := range m.faces {
		bounds, advance, ok = face.GlyphBounds(r)
		if ok {
			return
		}
	}
	return m.faces[m.preferred].GlyphBounds(r)
}

func (m *multi) GlyphAdvance(r rune) (
	advance fixed.Int26_6, ok bool,
) {
	for _, face := range m.faces {
		advance, ok = face.GlyphAdvance(r)
		if ok {
			return
		}
	}
	return m.faces[m.preferred].GlyphAdvance(r)
}

func (m *multi) Kern(r0, r1 rune) fixed.Int26_6 {
	return m.faces[m.preferred].Kern(r0, r1)
}

func (m *multi) Metrics() font.Metrics {
	return m.faces[m.preferred].Metrics()
}
