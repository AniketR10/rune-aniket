package cell

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/term"
)

func testSearch(t *testing.T, constructor func(*Buffer) Searcher) {
	t.Run("searches for occurrences of a word", func(t *testing.T) {
		r := NewBuffer()
		_, _ = r.ReadFrom(strings.NewReader("\thello"))
		s := constructor(r)
		require.Equal(t, 1, s.Search("hello"))

		res, ok := s.NextResult()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{X: 4}, res)
	})

	t.Run("searches for occurrences of a >1 width rune", func(t *testing.T) {
		r := NewBuffer()
		_, _ = r.ReadFrom(strings.NewReader("\nni hao!\n你好"))
		s := constructor(r)
		require.Equal(t, 1, s.Search("好"))

		res, ok := s.NextResult()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{X: 2, Y: 2}, res)
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
		assert.Equal(t, term.Coordinates{X: 4, Y: 1}, res)
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
