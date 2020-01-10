package cell

import (
	"fmt"
	"testing"

	"github.com/ernestrc/fractal/term"
	"github.com/stretchr/testify/assert"
)

var undoFortune = `Love in your heart wasn't put there to stay.
Love isn't love 'til you give it away.
		-- Oscar Hammerstein 中国`

func initUndoTestBuffer(t *testing.T) (u *undoer, b *Buffer) {
	b = newBufferWithContent(t, undoFortune)

	u = newUndoer(b.writer)
	b.writer = u
	return
}

func TestUndo(t *testing.T) {
	suite := []struct {
		name string
		cmd  func(b *Buffer)
	}{
		{"InsertAt", func(b *Buffer) {
			b.InsertAt(term.Coordinates{X: 0, Y: 2}, '\t')
		}},
		{"InsertRowAt", func(b *Buffer) {
			b.InsertRowAt(1)
		}},
		{"DeleteCell", func(b *Buffer) {
			b.DeleteCell(term.Coordinates{X: 4, Y: 2})
		}},
		{"ConflateRow", func(b *Buffer) {
			b.ConflateRow(1)
		}},
		{"TruncateRowFrom", func(b *Buffer) {
			b.TruncateRowFrom(term.Coordinates{X: 0, Y: 1})
		}},
		{"TruncateFrom", func(b *Buffer) {
			b.TruncateFrom(term.Coordinates{X: 6, Y: 0})
		}},
		{"DeleteRow", func(b *Buffer) {
			b.DeleteRow(0)
		}},
	}

	for _, _tcase := range suite {
		tcase := _tcase
		t.Run(fmt.Sprintf("undo %s", tcase.name), func(t *testing.T) {
			undoer, buf := initUndoTestBuffer(t)
			prev := buf.String()

			for i := 0; i < 5; i++ {
				tcase.cmd(buf)
				assert.True(t, undoer.undo())
			}

			after := buf.String()

			assert.Equal(t, prev, after)
		})
	}

	t.Run("undo/redo a series of updates", func(t *testing.T) {
		undoer, buf := initUndoTestBuffer(t)
		prev := buf.String()

		for _, tcase := range suite {
			t.Log(buf.String())
			tcase.cmd(buf)
		}

		middle := buf.String()

		for range suite {
			undoer.undo()
		}

		after := buf.String()
		assert.Equal(t, prev, after)

		for range suite {
			undoer.redo()
		}

		afterRedo := buf.String()
		assert.Equal(t, middle, afterRedo)

		for range suite {
			undoer.undo()
		}

		after = buf.String()
		assert.Equal(t, prev, after)
	})
}
