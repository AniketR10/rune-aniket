// Copyright (C) 2017-2026 Unstable Build, LLC
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/math/fixed"
)

func TestMultiFace(t *testing.T) {
	t.Run("Glyph keeps calling Glyph if faces return false, returns preferred", func(t *testing.T) {
		mock1 := mockFace{}
		mock2 := mockFace{}
		face := newMultiFace(0, &mock1, &mock2)

		face.Glyph(fixed.Point26_6{}, 'a')
		assert.Equal(t, 2, mock1.glyph)
		assert.Equal(t, 1, mock2.glyph)
	})

	t.Run("GlyphBouns keeps calling GlyphBounds if faces return false, returns preferred", func(t *testing.T) {
		mock1 := mockFace{}
		mock2 := mockFace{}
		face := newMultiFace(0, &mock1, &mock2)

		face.GlyphBounds('a')
		assert.Equal(t, 2, mock1.bounds)
		assert.Equal(t, 1, mock2.bounds)
	})

	t.Run("GlyphAdvance keeps calling GlyphAdvance if faces return false, returns preferred", func(t *testing.T) {
		mock1 := mockFace{}
		mock2 := mockFace{}
		face := newMultiFace(0, &mock1, &mock2)

		face.GlyphAdvance('a')
		assert.Equal(t, 2, mock1.advance)
		assert.Equal(t, 1, mock2.advance)
	})

	t.Run("Kern calls preferred face", func(t *testing.T) {
		mock1 := mockFace{}
		mock2 := mockFace{}
		face := newMultiFace(1, &mock1, &mock2)

		face.Kern('a', 'b')
		assert.Equal(t, 0, mock1.kern)
		assert.Equal(t, 1, mock2.kern)
	})

	t.Run("Metrics calls preferred face", func(t *testing.T) {
		mock1 := mockFace{}
		mock2 := mockFace{}
		face := newMultiFace(1, &mock1, &mock2)

		face.Metrics()
		assert.Equal(t, 0, mock1.metrics)
		assert.Equal(t, 1, mock2.metrics)
	})

	t.Run("Close dispatches Close to all underlying faces", func(t *testing.T) {
		mock1 := mockFace{}
		mock2 := mockFace{}
		face := newMultiFace(0, &mock1, &mock2)

		require.NoError(t, face.Close())
		assert.Equal(t, 1, mock1.close)
		assert.Equal(t, 1, mock2.close)
	})
}
