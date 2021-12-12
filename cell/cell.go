package cell

import (
	"fmt"

	"github.com/ernestrc/go-tui/term"
)

// Reader is the interface that wraps methods to query a 2D matrix of term.Cell.
type Reader interface {
	Rows() int
	Columns(row int) int
	Cell(term.Coordinates) (term.Cell, bool)
	RawCells() [][]term.Cell
	fmt.Stringer
}

// Writer is the interface that wraps methods to mutate a 2D matrix of term.Cell.
type Writer interface {
	// Insert inserts str at coordinates and returns the right-exclusive
	// coordinates of the actual insert range.
	Insert(at term.Coordinates, str string) (from, to term.Coordinates)
	// Delete deletes the cells between right-exclusive range defined by from, to.
	// It returns the true start and end coordinates of the delete operation,
	// which account for various artifacts like tab spaces.
	Delete(from, to term.Coordinates) (start, end term.Coordinates, str string)
}

// NewReader returns a new Reader which reads from cells and uses tabspaces.
func NewReader(cells [][]term.Cell, tabspaces int) Reader {
	r := &rawCells{cells: cells, tabspaces: tabspaces}
	return r
}
