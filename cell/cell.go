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
	Reset()
}
