package cell

import (
	"github.com/ernestrc/fractal/term"
)

// Reader is the interface that wraps methods to query a 2D matrix of term.Cell.
type Reader interface {
	Rows() int
	Columns(row int) int
	Cell(term.Coordinates) (term.Coordinates, term.Cell)
	RawCells() [][]term.Cell
	String() string
}

// Writer is the interface that wraps methods to mutate a 2D matrix of term.Cell.
type Writer interface {
	NextWrite() term.Coordinates
	Insert(at term.Coordinates, str string) (from, to term.Coordinates)
	Delete(from, to term.Coordinates) (start, end term.Coordinates, str string)
	Reset()
}

// ReadWriter is the interface that groups methods to query and manipulate
// a 2D matrix of term.Cell.
type ReadWriter interface {
	Writer
	Reader
}
