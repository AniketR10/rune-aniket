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
	// Update replaces any content from [start:end) with str and returns the
	// right-exclusive coordinates of the effective insert range. Note that
	// returned from, to values will be equal to each other if this operation
	// only removes content. This effectively allows clients to reverse a call
	// to Update by calling it again with the last return values.
	//
	// This method should panic if delete range between start, end is out of bounds.
	Update(start, end term.Coordinates, new string) (from, to term.Coordinates, old string)
}

// NewReader returns a new Reader which reads from cells and uses tabspaces.
func NewReader(cells [][]term.Cell, tabspaces int) Reader {
	r := &rawCells{cells: cells, tabspaces: tabspaces}
	return r
}
