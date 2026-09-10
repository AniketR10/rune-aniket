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

package cell

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func testSearch(t *testing.T, constructor func(*Buffer) Searcher) {
	t.Run("searches for occurrences of a word", func(t *testing.T) {
		r := NewBuffer()
		_, _ = r.ReadFrom(strings.NewReader("\thello"))
		s := constructor(r)
		require.Equal(t, 1, s.Search("hello"))

		res, ok := s.NextResult()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{X: 1}, res)
	})

	t.Run("searches for occurrences of a >1 width rune", func(t *testing.T) {
		r := NewBuffer()
		_, _ = r.ReadFrom(strings.NewReader("\nni hao!\n你好"))
		s := constructor(r)
		require.Equal(t, 1, s.Search("好"))

		res, ok := s.NextResult()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{X: 1, Y: 2}, res)
	})

	t.Run("searches for occurrences with multiple words", func(t *testing.T) {
		r := NewBuffer()
		_, _ = r.ReadFrom(strings.NewReader("a b\nc d e f g h i j k\n"))
		s := constructor(r)
		require.Equal(t, 1, s.Search(" g h i"))

		res, ok := s.NextResult()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{X: 7, Y: 1}, res)
	})

	t.Run("searches for occurrences with tabspaces", func(t *testing.T) {
		r := NewBuffer()
		_, _ = r.ReadFrom(strings.NewReader("a b\nc\td"))
		s := constructor(r)
		require.Equal(t, 1, s.Search("\td"))

		res, ok := s.NextResult()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{X: 1, Y: 1}, res)
	})

	t.Run("returns 0 if there are no matches", func(t *testing.T) {
		r := NewBuffer()
		_, _ = r.ReadFrom(strings.NewReader("\thello"))
		s := constructor(r)
		require.Equal(t, 0, s.Search("bollocks"))

		_, ok := s.NextResult()
		require.False(t, ok)
	})
}

func TestSimpleSearcher(t *testing.T) {
	testSearch(t, func(buf *Buffer) Searcher {
		return NewSimpleSearcher(buf)
	})
}
