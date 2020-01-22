package cell

import (
	"fmt"

	"github.com/ernestrc/fractal/term"
)

// reader is the interface that wraps methods to query a 2D matrix of term.Cell.
type reader interface {
	rows() int
	columns(row int) int
	cell(term.Coordinates) (term.Cell, bool)
	rawCells() [][]term.Cell
	fmt.Stringer
}

// writer is the interface that wraps methods to mutate a 2D matrix of term.Cell.
type writer interface {
	insert(at term.Coordinates, str string) (from, to term.Coordinates)
	delete(from, to term.Coordinates) (start, end term.Coordinates, str string)
	reset()
}
