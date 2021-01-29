package cell

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ernestrc/go-tui/term"
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
		{"Insert", func(b *Buffer) {
			b.Insert(term.Coordinates{X: 0, Y: 2}, '\t')
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
				ok, _ := undoer.undo()
				// TODO assert.Equal(t, term.Coordinates{}, at)
				assert.True(t, ok)
			}

			after := buf.String()

			assert.Equal(t, prev, after)
		})
	}

	t.Run("undo/redo a series of updates", func(t *testing.T) {
		undoer, buf := initUndoTestBuffer(t)
		prev := buf.String()

		for _, tcase := range suite {
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

func TestUndoEOL(t *testing.T) {
	const (
		filecontent1 = `package me.drton.jmavsim;
public class Rotor {
     sta  mtyp;


  myClass;
`
		insertStr = "\tmyClassVar\n"
	)

	insertAt := term.Coordinates{Y: 5, X: 9}
	abuf := NewBuffer()
	abuf.ReadFrom(strings.NewReader(filecontent1))
	abuf.WriteString("\n") //unix EOL
	astr0 := abuf.String()
	arcells0 := abuf.RawCells()

	afrom, ato := abuf.writer.Insert(insertAt, insertStr)
	astr1 := abuf.String()
	arcells1 := abuf.RawCells()

	abuf.writer.Delete(afrom, ato)
	astr2 := abuf.String()
	arcells2 := abuf.RawCells()
	assert.Equal(t, astr0, astr2)
	assert.Equal(t, arcells0, arcells2)

	ok, _ := abuf.Undo()
	assert.True(t, ok)
	bstr1 := abuf.String()
	brcells1 := abuf.RawCells()
	assert.Equal(t, astr1, bstr1)
	assert.Equal(t, arcells1, brcells1)

	ok, _ = abuf.Undo()
	assert.True(t, ok)
	bstr0 := abuf.String()
	brcells0 := abuf.RawCells()
	assert.Equal(t, astr0, bstr0)
	assert.Equal(t, arcells0, brcells0)

	ok, _ = abuf.Redo()
	assert.True(t, ok)
	ok, _ = abuf.Redo()
	assert.True(t, ok)
	bstr2 := abuf.String()
	brcells2 := abuf.RawCells()
	assert.Equal(t, astr2, bstr2)
	assert.Equal(t, arcells2, brcells2)
}
