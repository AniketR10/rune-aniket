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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/math/fixed"
)

func TestCacheFace(t *testing.T) {
	t.Run("caches calls to Glyph", func(t *testing.T) {
		mock := mockFace{}
		face := newCacheFace(&mock)

		for i := 0; i < 3; i++ {
			face.Glyph(fixed.Point26_6{}, 'a')
			assert.Equal(t, 1, mock.glyph)
		}
	})

	t.Run("caches calls to GlyphBounds", func(t *testing.T) {
		mock := mockFace{}
		face := newCacheFace(&mock)

		for i := 0; i < 3; i++ {
			face.GlyphBounds('a')
			assert.Equal(t, 1, mock.bounds)
		}
	})

	t.Run("caches calls to GlyphAdvance", func(t *testing.T) {
		mock := mockFace{}
		face := newCacheFace(&mock)

		for i := 0; i < 3; i++ {
			face.GlyphAdvance('a')
			assert.Equal(t, 1, mock.advance)
		}
	})

	t.Run("caches calls to Metrics", func(t *testing.T) {
		mock := mockFace{}
		face := newCacheFace(&mock)

		for i := 0; i < 3; i++ {
			face.Metrics()
			assert.Equal(t, 1, mock.metrics)
		}
	})

	t.Run("caches calls to Kern", func(t *testing.T) {
		mock := mockFace{}
		face := newCacheFace(&mock)

		for i := 0; i < 3; i++ {
			face.Kern('a', 'A')
			assert.Equal(t, 1, mock.kern)
		}
	})

	t.Run("Close dispatches Close to underlying face", func(t *testing.T) {
		mock := mockFace{}
		face := newCacheFace(&mock)

		require.NoError(t, face.Close())
		assert.Equal(t, 1, mock.close)
	})
}
