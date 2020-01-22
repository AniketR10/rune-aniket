package cell

import (
	"github.com/ernestrc/fractal/term"
)

// unixFileBuffer is a reader that hides the last EOL if present.
type unixFileBuffer struct {
	reader reader
}

func newUnixFileBuffer(r reader) reader {
	b := new(unixFileBuffer)
	b.reader = r
	return b
}

func (b *unixFileBuffer) endswithEOL() bool {
	rows := b.reader.rows()
	return rows != 0 && b.reader.columns(rows-1) == 0
}

func (b *unixFileBuffer) rows() (rows int) {
	rows = b.reader.rows()
	if !b.endswithEOL() {
		return
	}
	rows--
	return

}

func (b *unixFileBuffer) columns(row int) int {
	return b.reader.columns(row)
}

func (b *unixFileBuffer) cell(pos term.Coordinates) (term.Cell, bool) {
	return b.reader.cell(pos)
}

func (b *unixFileBuffer) rawCells() (cells [][]term.Cell) {
	cells = b.reader.rawCells()
	if !b.endswithEOL() {
		return
	}
	cells = cells[:len(cells)-1]
	return
}

func (b *unixFileBuffer) String() string {
	if !b.endswithEOL() {
		return b.reader.String()
	}
	return CellsToString(b.rawCells())
}
