package editor

import (
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

// unixFileReader is a reader that hides the last EOL if present.
type unixFileReader struct {
	reader cell.Reader
}

func newUnixFileReader(r cell.Reader) *unixFileReader {
	b := new(unixFileReader)
	b.reader = r
	return b
}

func (b *unixFileReader) endsWithEOL() bool {
	cells := b.reader.RawCells()
	return len(cells) > 1 && len(cells[len(cells)-1]) == 0
}

func (b *unixFileReader) Rows() (rows int) {
	rows = b.reader.Rows()
	if !b.endsWithEOL() {
		return
	}
	rows--
	return

}

func (b *unixFileReader) Columns(row int) int {
	return b.reader.Columns(row)
}

func (b *unixFileReader) Cell(pos term.Coordinates) (term.Cell, bool) {
	return b.reader.Cell(pos)
}

func (b *unixFileReader) RawCells() (cells [][]term.Cell) {
	cells = b.reader.RawCells()
	if !b.endsWithEOL() {
		return
	}
	cells = cells[:len(cells)-1]
	return
}

func (b *unixFileReader) String() string {
	if !b.endsWithEOL() {
		return b.reader.String()
	}
	return cell.CellsToString(b.RawCells())
}
