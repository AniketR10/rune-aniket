package screen

import (
	"testing"

	"github.com/rivo/uniseg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
)

func TestPrimaryCoordinates(t *testing.T) {
	b := NewPrimaryBuffer()
	assert.Equal(t, term.Coordinates{}, b.CursorAtScroll())
	assert.Equal(t, term.Coordinates{}, b.CursorAtScreen())

	b.Resize(5, 5)
	assert.Equal(t, term.Coordinates{}, b.CursorAtScroll())
	assert.Equal(t, term.Coordinates{}, b.CursorAtScreen())

	b.SetCursorAtScroll(term.Coordinates{Y: 5, X: 5}, false)
	assert.Equal(t, term.Coordinates{Y: 5, X: 5}, b.CursorAtScroll())
	assert.Equal(t, term.Coordinates{Y: 5, X: 5}, b.CursorAtScreen())

	b.SetOffset(term.Coordinates{Y: 1})
	assert.Equal(t, term.Coordinates{Y: 6, X: 5}, b.CursorAtScroll())
	assert.Equal(t, term.Coordinates{Y: 5, X: 5}, b.CursorAtScreen())

	b.MoveToOffset(term.Coordinates{Y: 0})
	assert.Equal(t, term.Coordinates{Y: 6, X: 5}, b.CursorAtScroll())
	assert.Equal(t, term.Coordinates{Y: 6, X: 5}, b.CursorAtScreen())

	b.MoveToOffset(term.Coordinates{Y: 1})
	assert.Equal(t, term.Coordinates{Y: 6, X: 5}, b.CursorAtScroll())
	assert.Equal(t, term.Coordinates{Y: 5, X: 5}, b.CursorAtScreen())

	b.SetOffset(term.Coordinates{Y: 0})
	assert.Equal(t, term.Coordinates{Y: 5, X: 5}, b.CursorAtScroll())
	assert.Equal(t, term.Coordinates{Y: 5, X: 5}, b.CursorAtScreen())
}

func TestPrimarySelection(t *testing.T) {
	t.Run("select with scroll", func(t *testing.T) {
		b := makePrimaryBufferForTesting(1, 5)
		resetPrimaryBuffer(t, b, "0\n1\n2\n3\n4\n5\n6\n7\n8\n9")

		b.Select(term.Coordinates{Y: 6})
		b.SelectEnd(term.Coordinates{Y: 7})
		cells, ok := b.Selection()
		require.True(t, ok)
		assert.Equal(t, [][]term.Cell{{{Ch: '6', Width: 1}}, {{Ch: '7', Width: 1}}}, cells)

		b.SetOffset(term.Coordinates{Y: 5})
		cells, ok = b.Selection()
		require.True(t, ok)
		assert.Equal(t, [][]term.Cell{{{Ch: '6', Width: 1}}, {{Ch: '7', Width: 1}}}, cells)
	})
	t.Run("word selection with scrollback history", func(t *testing.T) {
		b := makePrimaryBufferForTesting(2, 10)
		resetPrimaryBuffer(t, b, "00\n11\n22\n33\n44\n55\n66\n77\n88\n99")

		b.SetOffset(term.Coordinates{Y: 2})
		b.SelectWordAt(term.Coordinates{})
		cells, ok := b.Selection()
		require.True(t, ok)
		assert.Equal(t, [][]term.Cell{{{Ch: '2', Width: 1}, {Ch: '2', Width: 1}}}, cells)
	})

	t.Run("coordinates with scroll", func(t *testing.T) {
		b := makePrimaryBufferForTesting(1, 5)
		resetPrimaryBuffer(t, b, "0\n1\n2\n3\n4\n5\n6\n7\n8\n9")

		b.Select(term.Coordinates{Y: 6})
		b.SelectEnd(term.Coordinates{Y: 7})

		from, to, ok := b.SelectionCoordinatesAtScreen()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{Y: 6}, from)
		assert.Equal(t, term.Coordinates{Y: 7, X: 1}, to)

		from, to, ok = b.SelectionCoordinatesAtScroll()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{Y: 6}, from)
		assert.Equal(t, term.Coordinates{Y: 7, X: 1}, to)

		b.SetOffset(term.Coordinates{Y: 5})

		from, to, ok = b.SelectionCoordinatesAtScreen()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{Y: 1}, from)
		assert.Equal(t, term.Coordinates{Y: 2, X: 1}, to)

		from, to, ok = b.SelectionCoordinatesAtScroll()
		require.True(t, ok)
		assert.Equal(t, term.Coordinates{Y: 6}, from)
		assert.Equal(t, term.Coordinates{Y: 7, X: 1}, to)
	})
}

func makePrimaryBufferForTesting(width, height int) *PrimaryBuffer {
	ret := NewPrimaryBuffer()
	ret.Resize(width, height)
	return ret
}

func writeToPrimaryBuffer(b *PrimaryBuffer, str string) {
	for _, ch := range str {
		pos := b.CursorAtScreen()
		if ch == '\n' {
			pos.Y++
			pos.X = 0
			b.SetCursorAtScreen(pos, false)
		} else {
			b.Write(ch, uniseg.StringWidth(string(ch)), 0)
			pos.X++
			b.SetCursorAtScreen(pos, false)
		}
	}
}

func resetPrimaryBuffer(t *testing.T, b *PrimaryBuffer, to string) {
	b.ResetLines(0, b.Height())
	b.SetCursorAtScreen(term.Coordinates{}, false)
	writeToPrimaryBuffer(b, to)
	require.Equal(t, to, cell.CellsToString(b.Cells.RawCells()))
}
