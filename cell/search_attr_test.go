package cell

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/term"
)

func TestAttrSearcherSearch(t *testing.T) {
	fn := func(buf *Buffer) Searcher {
		searcher := NewSimpleSearcher(buf)
		s := AttrSearcher(searcher, buf, term.Attributes{})
		return s
	}
	testSearch(t, fn)
}

func newAttrSearcher(t *testing.T, content string) (*Buffer, SubscriberSearcher) {
	buf := NewBuffer()
	searcher := NewSimpleSearcher(buf)
	s := AttrSearcher(searcher, buf, term.Attributes{Bg: term.ColorRed, Fg: term.ColorCyan})
	_, err := buf.ReadFrom(strings.NewReader(content))
	require.NoError(t, err)
	buf.Subscribe(s)

	return buf, s
}

func TestAttrSearcher(t *testing.T) {

	t.Run("sets/unsets attributes upon Search", func(t *testing.T) {
		buf, s := newAttrSearcher(t, "yo wasup")

		require.Equal(t, 1, s.Search("wasup"))
		expected := [][]term.Cell{
			{
				{Ch: 'y'},
				{Ch: 'o'},
				{Ch: ' '},
				{Ch: 'w', Bg: term.ColorRed, Fg: term.ColorCyan},
				{Ch: 'a', Bg: term.ColorRed, Fg: term.ColorCyan},
				{Ch: 's', Bg: term.ColorRed, Fg: term.ColorCyan},
				{Ch: 'u', Bg: term.ColorRed, Fg: term.ColorCyan},
				{Ch: 'p', Bg: term.ColorRed, Fg: term.ColorCyan},
			},
		}
		assert.Equal(t, expected, buf.RawCells())

		require.Equal(t, 0, s.Search(""))
		expected = [][]term.Cell{
			{
				{Ch: 'y'}, {Ch: 'o'}, {Ch: ' '},
				{Ch: 'w'}, {Ch: 'a'}, {Ch: 's'},
				{Ch: 'u'}, {Ch: 'p'},
			},
		}
		assert.Equal(t, expected, buf.RawCells())
	})

	t.Run("unsets attributes if match result is partially deleted", func(t *testing.T) {
		buf, s := newAttrSearcher(t, "yo wasup")
		require.Equal(t, 1, s.Search("wasup"))

		buf.DeleteCell(term.Coordinates{X: 7})
		require.Equal(t, "yo wasu", buf.String())

		expected := [][]term.Cell{
			{
				{Ch: 'y'}, {Ch: 'o'}, {Ch: ' '},
				{Ch: 'w'}, {Ch: 'a'}, {Ch: 's'},
				{Ch: 'u'},
			},
		}
		assert.Equal(t, expected, buf.RawCells())
	})

	t.Run("unsets attributes of ONE of the match results if it is partially deleted", func(t *testing.T) {
		buf, s := newAttrSearcher(t, "yo yo wasup")
		require.Equal(t, 2, s.Search("yo"))

		buf.DeleteCell(term.Coordinates{X: 0})
		require.Equal(t, "o yo wasup", buf.String())

		expected := [][]term.Cell{
			{
				{Ch: 'o'}, {Ch: ' '},
				{Ch: 'y', Bg: term.ColorRed, Fg: term.ColorCyan},
				{Ch: 'o', Bg: term.ColorRed, Fg: term.ColorCyan},
				{Ch: ' '},
				{Ch: 'w'}, {Ch: 'a'}, {Ch: 's'},
				{Ch: 'u'}, {Ch: 'p'},
			},
		}
		assert.Equal(t, expected, buf.RawCells())
	})

	t.Run("unsets attributes of ONE of the match results if it is inserted in the middle", func(t *testing.T) {
		buf, s := newAttrSearcher(t, "yo yo wasup")
		require.Equal(t, 2, s.Search("yo"))

		buf.Insert(term.Coordinates{X: 1}, 'j')
		require.Equal(t, "yjo yo wasup", buf.String())

		expected := [][]term.Cell{
			{
				{Ch: 'y'}, {Ch: 'j'},
				{Ch: 'o'}, {Ch: ' '},
				{Ch: 'y', Bg: term.ColorRed, Fg: term.ColorCyan},
				{Ch: 'o', Bg: term.ColorRed, Fg: term.ColorCyan},
				{Ch: ' '},
				{Ch: 'w'}, {Ch: 'a'}, {Ch: 's'},
				{Ch: 'u'}, {Ch: 'p'},
			},
		}
		assert.Equal(t, expected, buf.RawCells())
	})

	t.Run("sets attributes if text is added such that there's a new match result ", func(t *testing.T) {
		buf, s := newAttrSearcher(t, "yo wasup")

		require.Equal(t, 0, s.Search("wasupp"))

		buf.Insert(term.Coordinates{X: 8}, 'p')
		require.Equal(t, "yo wasupp", buf.String())

		expected := [][]term.Cell{
			{
				{Ch: 'y'},
				{Ch: 'o'},
				{Ch: ' '},
				{Ch: 'w', Bg: term.ColorRed, Fg: term.ColorCyan},
				{Ch: 'a', Bg: term.ColorRed, Fg: term.ColorCyan},
				{Ch: 's', Bg: term.ColorRed, Fg: term.ColorCyan},
				{Ch: 'u', Bg: term.ColorRed, Fg: term.ColorCyan},
				{Ch: 'p', Bg: term.ColorRed, Fg: term.ColorCyan},
				{Ch: 'p', Bg: term.ColorRed, Fg: term.ColorCyan},
			},
		}
		assert.Equal(t, expected, buf.RawCells())
	})

	t.Run("unsets attributes if match result is inserted in the middle such that there's no more match", func(t *testing.T) {
		buf, s := newAttrSearcher(t, "yo wasup")

		require.Equal(t, 1, s.Search("wasup"))

		buf.Insert(term.Coordinates{X: 5}, 'p')
		require.Equal(t, "yo wapsup", buf.String())

		expected := [][]term.Cell{
			{
				{Ch: 'y'},
				{Ch: 'o'},
				{Ch: ' '},
				{Ch: 'w'},
				{Ch: 'a'},
				{Ch: 'p'},
				{Ch: 's'},
				{Ch: 'u'},
				{Ch: 'p'},
			},
		}
		assert.Equal(t, expected, buf.RawCells())
	})

	t.Run("sets attributes if text is deleted in the middle such that text now matches", func(t *testing.T) {
		buf, s := newAttrSearcher(t, "yo wasup")

		require.Equal(t, 1, s.Search("wasup"))

		buf.DeleteCell(term.Coordinates{X: 5})
		require.Equal(t, "yo waup", buf.String())

		expected := [][]term.Cell{
			{
				{Ch: 'y'},
				{Ch: 'o'},
				{Ch: ' '},
				{Ch: 'w'},
				{Ch: 'a'},
				{Ch: 'u'},
				{Ch: 'p'},
			},
		}
		assert.Equal(t, expected, buf.RawCells())
	})
}
