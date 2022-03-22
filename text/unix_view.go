package text

import (
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

// unixFileView is a reader that hides the last EOL if present.
type unixFileView struct {
	reader cell.View
}

func newUnixFileReader(r cell.View) *unixFileView {
	b := new(unixFileView)
	b.reader = r
	return b
}

func (b *unixFileView) endsWithEOL() bool {
	cells := b.reader.RawCells()
	return len(cells) > 1 && len(cells[len(cells)-1]) == 0
}

func (b *unixFileView) Rows() (rows int) {
	rows = b.reader.Rows()
	if !b.endsWithEOL() {
		return
	}
	rows--
	return

}

func (b *unixFileView) Columns(row int) int {
	return b.reader.Columns(row)
}

func (b *unixFileView) Cell(pos term.Coordinates) (term.Cell, bool) {
	return b.reader.Cell(pos)
}

func (b *unixFileView) RawCells() (cells [][]term.Cell) {
	cells = b.reader.RawCells()
	if !b.endsWithEOL() {
		return
	}
	cells = cells[:len(cells)-1]
	return
}

func (b *unixFileView) String() string {
	if !b.endsWithEOL() {
		return b.reader.String()
	}
	return cell.CellsToString(b.RawCells())
}
