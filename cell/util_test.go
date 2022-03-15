package cell

import (
	"strings"
	"testing"

	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertCoordinates(t *testing.T) {
	tsuite := []struct {
		cells [][]term.Cell
		y, x  int
		out   term.Coordinates
		ok    bool
	}{
		{nil, 0, 0, term.Coordinates{}, false},
		{[][]term.Cell{{{Ch: 'a'}}}, 0, 0, term.Coordinates{}, true},
		{[][]term.Cell{{{Ch: 0}, {Ch: 0}, {Ch: 0}, {Ch: '\t'}, {Ch: 'a'}}}, 0, 1, term.Coordinates{X: 4}, true},
		{[][]term.Cell{{}, {{Ch: 0}, {Ch: 0}, {Ch: 0}, {Ch: '\t'}, {Ch: 'a'}, {Ch: 0}}}, 1, 1, term.Coordinates{Y: 1, X: 4}, true},
	}

	for _, tcase := range tsuite {
		out, ok := ConvertRuneCoordinates(tcase.cells, tcase.y, tcase.x)
		require.Equal(t, tcase.ok, ok)
		assert.Equal(t, tcase.out, out)
	}
	for _, tcase := range tsuite {
		y, x, ok := ConvertTermCoordinates(tcase.cells, tcase.out)
		require.Equal(t, tcase.ok, ok)
		assert.Equal(t, tcase.x, x)
		assert.Equal(t, tcase.y, y)
	}
}
func TestCellsToBufferZero(t *testing.T) {
	t.Run("returned buffer should always have at least one row", func(t *testing.T) {
		buf := CellsToBuffer(nil)
		require.Equal(t, 1, buf.Rows())
		assert.Equal(t, 0, buf.Columns(0))
	})
}

func TestCellsToBuffer(t *testing.T) {
	const (
		filecontent1 = `package me.drton.jmavsim;
public class Rotor {
     sta	tmtyp;


  myClass;
`
		insertStr = "\tmyClassVar\n"
	)
	insertAt := term.Coordinates{Y: 5, X: 9}

	abuf := NewBuffer()
	abuf.ReadFrom(strings.NewReader(filecontent1))
	astr0 := abuf.String()
	arcells0 := abuf.RawCells()

	bbuf := CellsToBuffer(arcells0)
	bstr0 := bbuf.String()
	brcells0 := bbuf.RawCells()
	require.Equal(t, arcells0, brcells0)

	afrom, ato := abuf.writer.Insert(insertAt, insertStr)
	astr1 := abuf.String()
	arcells1 := abuf.RawCells()

	abuf.Delete(afrom, ato)
	astr2 := abuf.String()
	arcells2 := abuf.RawCells()

	bfrom, bto := bbuf.writer.Insert(insertAt, insertStr)
	bstr1 := bbuf.String()
	brcells1 := bbuf.RawCells()

	bbuf.Delete(bfrom, bto)
	bstr2 := bbuf.String()
	brcells2 := bbuf.RawCells()

	require.Equal(t, filecontent1, astr0)
	require.Equal(t, filecontent1, bstr0)

	assert.Equal(t, astr1, bstr1)
	assert.Equal(t, arcells1, brcells1)

	assert.Equal(t, astr2, bstr2)
	assert.Equal(t, arcells2, brcells2)
}

func benchmarkCellToBuffer(b *testing.B, n int) {
	c := make([][]term.Cell, n)
	for i := 0; i < n; i++ {
		c[i] = make([]term.Cell, n)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CellsToBuffer(c)
	}
}

func BenchmarkCellToBuffer10(b *testing.B) {
	benchmarkCellToBuffer(b, 10)
}

func BenchmarkCellToBuffer100(b *testing.B) {
	benchmarkCellToBuffer(b, 100)
}

func BenchmarkCellToBuffer1000(b *testing.B) {
	benchmarkCellToBuffer(b, 1000)
}

func BenchmarkCellToBuffer10000(b *testing.B) {
	benchmarkCellToBuffer(b, 10000)
}
