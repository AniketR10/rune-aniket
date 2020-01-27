package cell

import (
	"strings"
	"testing"

	"github.com/ernestrc/fractal/term"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const str = "hello\n\tworld\n"
const longStr = `Love in your heart wasn't put there to stay.
Love isn't love 'til you give it away.
		-- Oscar Hammerstein 中国`

func newBufferWithContent(t *testing.T, str string) *Buffer {
	b := NewBuffer()
	_, err := b.ReadFrom(strings.NewReader(str))
	require.NoError(t, err)

	return b
}

func TestBufferInsertAt(t *testing.T) {
	buf := NewBuffer()
	var next term.Coordinates

	next = buf.InsertAt(next, 'h')
	next = buf.InsertAt(next, 'e')
	next = buf.InsertAt(next, 'l')
	next = buf.InsertAt(next, 'l')
	next = buf.InsertAt(next, 'o')
	next = buf.InsertAt(next, '\n')
	next = buf.InsertAt(next, 'w')
	next = buf.InsertAt(next, 'o')
	next = buf.InsertAt(next, 'r')
	next = buf.InsertAt(next, 'l')
	buf.InsertAt(next, 'd')
	next = buf.InsertAt(term.Coordinates{X: 4, Y: 0}, '\n')

	str := "hell\no\nworld"
	assert.Equal(t, 3, buf.Rows())

	cols := buf.Columns(0)
	assert.Equal(t, 4, cols)

	cols = buf.Columns(1)
	assert.Equal(t, 1, cols)

	cols = buf.Columns(2)
	assert.Equal(t, 5, cols)

	assert.Equal(t, str, buf.String())

	assert.Equal(t, term.Coordinates{Y: 1}, next)

	next = buf.InsertAt(term.Coordinates{Y: 1}, '\t')
	assert.Equal(t, term.Coordinates{Y: 1, X: 4}, next)
}

func TestBufferDeleteRow(t *testing.T) {
	str := "hello\nworld"
	buf := newBufferWithContent(t, str)

	assert.True(t, buf.DeleteRow(0))
	assert.Equal(t, "world", buf.String())

	assert.True(t, buf.DeleteRow(0))
	assert.Equal(t, "", buf.String())
}

func TestBufferDeleteRow2(t *testing.T) {
	str := "\nworld"
	buf := newBufferWithContent(t, str)

	assert.True(t, buf.DeleteRow(0))
	assert.Equal(t, "world", buf.String())
}

func TestBufferDeleteRowNotPanic(t *testing.T) {
	str := "1234"
	buf := newBufferWithContent(t, str)

	assert.False(t, buf.DeleteRow(1))
	assert.Equal(t, "1234", buf.String())
}

func TestBufferTruncateRowFrom1(t *testing.T) {
	str := "hello\nworld"
	buf := newBufferWithContent(t, str)

	buf.TruncateRowFrom(term.Coordinates{X: 2, Y: 0})

	assert.Equal(t, "he\nworld", buf.String())
}

func TestBufferTruncateRowFrom2(t *testing.T) {
	str := "hello\nworld"
	buf := newBufferWithContent(t, str)

	buf.TruncateRowFrom(term.Coordinates{X: 2, Y: 1})

	assert.Equal(t, "hello\nwo", buf.String())
}

func TestBufferTruncateCellAt(t *testing.T) {
	str := "hello\nworld"
	buf := newBufferWithContent(t, str)

	buf.DeleteCell(term.Coordinates{X: 0, Y: 1})
	buf.DeleteCell(term.Coordinates{X: 1, Y: 1})
	buf.ConflateRow(0)
	buf.DeleteCell(term.Coordinates{X: 7, Y: 0})

	assert.Equal(t, "hellool", buf.String())
}

func TestBufferDeleteCellAtTab(t *testing.T) {
	str := "!\t\t!\t"
	buf := newBufferWithContent(t, str)
	require.Equal(t, 14, buf.Columns(0))

	start, ok := buf.DeleteCell(term.Coordinates{X: 3, Y: 0})
	assert.True(t, ok)
	assert.Equal(t, term.Coordinates{X: 1}, start)
	assert.Equal(t, "!\t!\t", buf.String())
	assert.Equal(t, 10, buf.Columns(0))

	start, ok = buf.DeleteCell(term.Coordinates{X: 2, Y: 0})
	assert.True(t, ok)
	assert.Equal(t, term.Coordinates{X: 1}, start)
	assert.Equal(t, "!!\t", buf.String())
	assert.Equal(t, 6, buf.Columns(0))

	start, ok = buf.DeleteCell(term.Coordinates{X: 2, Y: 0})
	assert.True(t, ok)
	assert.Equal(t, term.Coordinates{X: 2}, start)
	assert.Equal(t, "!!", buf.String())
	assert.Equal(t, 2, buf.Columns(0))

	str = "\t\t>>>>"
	buf = newBufferWithContent(t, str)
	start, ok = buf.DeleteCell(term.Coordinates{X: 3, Y: 0})
	assert.True(t, ok)
	assert.Equal(t, term.Coordinates{X: 0}, start)
	assert.Equal(t, "\t>>>>", buf.String())
	assert.Equal(t, 8, buf.Columns(0))
}

type selectCase struct {
	from     term.Coordinates
	to       term.Coordinates
	expected [][]term.Cell
}

func toString(cells [][]term.Cell) string {
	runes := make([]rune, 0)
	for _, r := range cells {
		for _, c := range r {
			runes = append(runes, c.Ch)
		}
	}
	return string(runes)
}

func assertCellProperties(t *testing.T, cell term.Cell, attr term.Attributes) {
	assert.Equal(t, attr.Fg, cell.Fg)
	assert.Equal(t, attr.Bg, cell.Bg)
}

func TestSetAttr(t *testing.T) {
	str := "0123"
	buf := newBufferWithContent(t, str)

	t.Run("sets attribute to cell at position if exists", func(t *testing.T) {
		pos := term.Coordinates{X: 0, Y: 0}
		attr := term.Attributes{Fg: term.AttrBold, Bg: term.AttrReverse}
		assert.True(t, buf.SetAttr(pos, attr))
		c, ok := buf.Cell(pos)
		assert.True(t, ok)
		assertCellProperties(t, c, attr)
	})
	t.Run("returns ok=false if cell at position does not exist", func(t *testing.T) {
		coords := []term.Coordinates{
			term.Coordinates{X: 10, Y: 10},
			term.Coordinates{X: 0, Y: 10},
			term.Coordinates{X: 4, Y: 0},
		}
		for _, pos := range coords {
			attr := term.Attributes{Fg: term.AttrBold, Bg: term.AttrReverse}
			assert.False(t, buf.SetAttr(pos, attr))
		}
	})
}

func TestBufferTruncateFrom(t *testing.T) {

	tsuite := []struct {
		contents string
		input    term.Coordinates
		ok       bool
		expected string
	}{
		{"hello\nworld\n", term.Coordinates{X: 4, Y: 0}, true, "hell"},
		{"hello\nworld\n", term.Coordinates{X: 0, Y: 1}, true, "hello"},
		{longStr, term.Coordinates{X: 6, Y: 0}, true, "Love i"},
	}

	for _, tcase := range tsuite {
		buf := newBufferWithContent(t, tcase.contents)
		ok := buf.TruncateFrom(tcase.input)
		if tcase.ok {
			assert.True(t, ok)
			assert.Equal(t, tcase.expected, buf.String())
		} else {
			assert.False(t, ok)
		}
	}
}

func TestBufferDelete(t *testing.T) {
	b := NewBuffer()
	_, err := b.ReadFrom(strings.NewReader("bla\nbleh"))
	require.NoError(t, err)

	start, end, str := b.Delete(term.Coordinates{}, term.Coordinates{X: 3})
	assert.Equal(t, term.Coordinates{}, start)
	assert.Equal(t, term.Coordinates{X: 3}, end)
	assert.Equal(t, "bla\n", str)
}
