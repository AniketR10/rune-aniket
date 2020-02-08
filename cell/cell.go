package cell

import (
	"fmt"

	"github.com/ernestrc/fractal/term"
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
	Insert(at term.Coordinates, str string) (from, to term.Coordinates)
	Delete(from, to term.Coordinates) (start, end term.Coordinates, str string)
}

// Subscriber is the interface that wraps methods to subscribe to updates to
// an underlying cell.Writer.
//
// Subscribers MUST NOT have mutable access to the underlying cell.Writer
// they're subscribing to, as updates are published synchronously so
// program could enter in an infinite loop.
type Subscriber interface {
	OnInsert(from, to term.Coordinates, str string)
	OnDelete(start, end term.Coordinates, str string)
}

// NewReader returns a new Reader which reads from cells and uses tabspaces.
func NewReader(cells [][]term.Cell, tabspaces int) Reader {
	r := &rawCells{cells: cells, tabspaces: tabspaces}
	return r
}
