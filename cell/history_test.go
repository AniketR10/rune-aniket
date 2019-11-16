package cell

// import (
// 	"fmt"
// 	"strings"
// 	"testing"
//
// 	"github.com/google/go-cmp/cmp"
// 	termbox "github.com/nsf/termbox-go"
// )
//
// var undoFortune = `Love in your heart wasn't put there to stay.
// Love isn't love 'til you give it away.
// 		-- Oscar Hammerstein 中国`
//
// func initUndoTestBuffer(t *testing.T) (b *UndoBuffer, bb Buffer) {
// 	_, err := bb.ReadFrom(strings.NewReader(undoFortune))
// 	if err != nil {
// 		t.Fatal(err)
// 	}
//
// 	b = NewUndoBuffer(&bb)
// 	return
// }
//
// func deepCloneCells(cells [][]termbox.Cell) [][]termbox.Cell {
// 	res := make([][]termbox.Cell, len(cells))
// 	for i, row := range cells {
// 		res[i] = make([]termbox.Cell, len(row))
// 		copy(res[i], row)
// 	}
// 	return res
// }
//
// // TODO
// func TestUndoAsymetrical(t *testing.T) {
// 	// WriteAt
// 	// InsertAt
// 	// InsertRowAt
// }
//
// func TestUndoSymmetrical(t *testing.T) {
// 	suite := []struct {
// 		name string
// 		cmd  func(b *UndoBuffer)
// 	}{
// 		{"InsertAt", func(b *UndoBuffer) {
// 			b.InsertAt(Coordinates{X: 0, Y: 2}, '\t')
// 		}},
// 		{"InsertRowAt", func(b *UndoBuffer) {
// 			b.InsertRowAt(1)
// 		}},
// 		{"SetAttr", func(b *UndoBuffer) {
// 			b.SetAttr(Coordinates{}, termbox.AttrBold, termbox.AttrBold)
// 		}},
// 		{"WriteAt", func(b *UndoBuffer) {
// 			b.WriteAt(Coordinates{X: 2, Y: 2}, 'f')
// 		}},
// 		{"WriteRune", func(b *UndoBuffer) {
// 			b.WriteRune('\t')
// 		}},
// 		{"WriteString", func(b *UndoBuffer) {
// 			b.WriteString("fjlkfjlkw\njfkelwjflkew\tjfklewjflkew\t")
// 		}},
// 		{"TruncateCellAt", func(b *UndoBuffer) {
// 			b.TruncateCellAt(Coordinates{X: 4, Y: 2})
// 		}},
// 		{"ConflateRow", func(b *UndoBuffer) {
// 			b.ConflateRow(1)
// 		}},
// 		{"TruncateRowFrom", func(b *UndoBuffer) {
// 			b.TruncateRowFrom(Coordinates{X: 0, Y: 1})
// 		}},
// 		{"TruncateFrom", func(b *UndoBuffer) {
// 			b.TruncateFrom(Coordinates{X: 6, Y: 0})
// 		}},
// 		{"TruncateRowAt", func(b *UndoBuffer) {
// 			b.TruncateRowAt(0)
// 		}},
// 	}
//
// 	for _, _tcase := range suite {
// 		tcase := _tcase
// 		t.Run(fmt.Sprintf("undo %s", tcase.name), func(t *testing.T) {
// 			b, bb := initUndoTestBuffer(t)
// 			prev := deepCloneCells(bb.RawCells())
// 			tcase.cmd(b)
// 			middle := deepCloneCells(bb.RawCells())
// 			b.Undo()
// 			after := deepCloneCells(bb.RawCells())
//
// 			if cmp.Equal(prev, middle) {
// 				t.Errorf("expected %s to update buffer but it didn't", tcase.name)
// 			}
//
// 			if !cmp.Equal(prev, after) {
// 				t.Errorf(cmp.Diff(prev, after))
// 			}
// 		})
// 	}
// 	t.Run("undo a series of updates", func(t *testing.T) {
// 		// FIXME
// 		t.SkipNow()
// 		b, bb := initUndoTestBuffer(t)
// 		prev := bb.String()
//
// 		for _, tcase := range suite {
// 			t.Logf("buffer BEFORE %s", tcase.name)
// 			t.Log(bb.String())
// 			tcase.cmd(b)
// 			t.Logf("buffer AFTER %s", tcase.name)
// 			t.Log(bb.String())
// 		}
// 		for i := range suite {
// 			t.Logf("buffer BEFORE undo %d", i)
// 			t.Log(bb.String())
// 			b.Undo()
// 			t.Logf("buffer AFTER undo %d", i)
// 			t.Log(bb.String())
// 		}
//
// 		after := bb.String()
//
// 		if prev != after {
// 			t.Errorf("expected:\n------ \n%s\n------ \n VS \n------\n%s\n------ \n", after, prev)
// 		}
// 	})
// }
