package cell

import (
	"github.com/ernestrc/fractal/term"
)

// unixFileBuffer is a reader that hides the last EOL if present.
type unixFileBuffer struct {
	reader Reader
}

func newUnixFileBuffer(r Reader) Reader {
	b := new(unixFileBuffer)
	b.reader = r
	return b
}

func (b *unixFileBuffer) endswithEOL() bool {
	rows := b.reader.Rows()
	return rows != 0 && b.reader.Columns(rows-1) == 0
}

func (b *unixFileBuffer) Rows() (rows int) {
	rows = b.reader.Rows()
	if !b.endswithEOL() {
		return
	}
	rows--
	return

}

func (b *unixFileBuffer) Columns(row int) int {
	return b.reader.Columns(row)
}

func (b *unixFileBuffer) Cell(pos term.Coordinates) (term.Cell, bool) {
	return b.reader.Cell(pos)
}

func (b *unixFileBuffer) RawCells() (cells [][]term.Cell) {
	cells = b.reader.RawCells()
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
	return CellsToString(b.RawCells())
}
