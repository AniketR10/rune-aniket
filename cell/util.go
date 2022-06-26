package cell

import (
	"strings"

	"github.com/ernestrc/go-tui/term"
)

// SortFromToBlock sorts a pair of coordinates (from/to) such that:
//
//  				cases
//
//  	┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐
//  	│ f    ││ t    ││    f ││    t ││ t  f ││ f  t │
//  	│    t ││    f ││ t    ││ f    ││      ││      │
//  	└──────┘└──────┘└──────┘└──────┘└──────┘└──────┘
//         |       |        |       |       |       |
//         v       v        v       v       v       v
//  	┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐
//  	│ f    ││ f    ││ f    ││ f    ││ f  t ││ f  t │
//  	│    t ││    t ││    t ││    t ││      ││      │
//  	└──────┘└──────┘└──────┘└──────┘└──────┘└──────┘
//
func SortFromToBlock(from term.Coordinates, to term.Coordinates) (
	term.Coordinates, term.Coordinates,
) {
	if from.X > to.X {
		temp := from.X
		from.X = to.X
		to.X = temp
	}
	if from.Y > to.Y {
		temp := from.Y
		from.Y = to.Y
		to.Y = temp
	}
	return from, to
}

// SortFromTo sorts a pair of coordinates (from/to) such that:
//
// 				cases
//
// 	┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐
// 	│ f    ││ t    ││    f ││    t ││ t  f ││ f  t │
// 	│    t ││    f ││ t    ││ f    ││      ││      │
// 	└──────┘└──────┘└──────┘└──────┘└──────┘└──────┘
//        |       |        |       |       |       |
//        v       v        v       v       v       v
// 	┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐┌──────┐
// 	│ f    ││ f    ││    f ││    f ││ f  t ││ f  t │
// 	│    t ││    t ││ t    ││ t    ││      ││      │
// 	└──────┘└──────┘└──────┘└──────┘└──────┘└──────┘
func SortFromTo(from term.Coordinates, to term.Coordinates) (
	term.Coordinates, term.Coordinates,
) {
	if from.Y > to.Y {
		temp := from.Y
		from.Y = to.Y
		to.Y = temp

		temp = from.X
		from.X = to.X
		to.X = temp
	} else if from.Y == to.Y && from.X > to.X {
		temp := from.X
		from.X = to.X
		to.X = temp
	}
	return from, to
}

// CellsToString returns the string representation of the given cell matrix.
func CellsToString(cells [][]term.Cell) string {
	builder := strings.Builder{}
	copyToBuilder(&builder, cells)
	return builder.String()
}

// StringToCells returns the cell matrix representation of the given string.
func StringToCells(str string, tabspaces int) (cells [][]term.Cell) {
	var builder rawCells
	builder.init(tabspaces)
	builder.ReadFrom(strings.NewReader(str))
	return builder.RawCells()
}

// CloneCells returns a deep clone of in.
func CloneCells(in [][]term.Cell) [][]term.Cell {
	ret := make([][]term.Cell, len(in))
	for i, r := range in {
		ret[i] = make([]term.Cell, len(r))
		copy(ret[i], r)
	}
	return ret
}

// CellsToBuffer efficienty returns a Buffer that uses c as the
// underlying matrix of cells.
//
// Note that this buffer will honor the tabspaces observed in c.
// If c does not have any tabspaces, then 1 tabspace is assumed.
//
// Furthermore, it won't treat the last EOL as mandatory so it can be used
// as an in-memory buffer.
func CellsToBuffer(c [][]term.Cell, tabspaces int) *Buffer {
	cells := new(rawCells)

	cells.init(tabspaces)
	cells.cells = CloneCells(c)

	// rawCells hasthe property that there's always at least one row
	if cells.Rows() == 0 {
		cells.fillInRows(0)
	}

	ret := new(Buffer)
	ret.initWithCells(cells, nil)
	return ret
}

// ConvertRuneCoordinates converts x and y, which use the buffer runes as offsets
// into term.Coordinates, which account for tab expansion. It returns false if y is out
// of bounds.
func ConvertRuneCoordinates(cells [][]term.Cell, y, x int) (
	ret term.Coordinates, ok bool,
) {
	if y > len(cells) || len(cells) == 0 {
		return
	}

	ret = term.Coordinates{Y: y, X: x}

	if ret.Y == len(cells) {
		ok = ret.X == 0
		return
	}

	line := cells[ret.Y]

	for xi, c := range line {
		if xi == ret.X {
			break
		}
		if c.Ch == 0 {
			ret.X++
		}
	}

	ok = true
	return
}

// ConvertTermCoordinates converts c, which use the terminal system of coordinates, which
// account for tab expansion, into rune offsets. It returns false if y is out
// of bounds.
func ConvertTermCoordinates(cells [][]term.Cell, c term.Coordinates) (y, x int, ok bool) {
	if c.Y > len(cells) || len(cells) == 0 {
		return
	}

	y = c.Y
	x = c.X

	if c.Y == len(cells) {
		ok = c.X == 0
		return
	}

	line := cells[c.Y]

	for xi, cell := range line {
		if xi == c.X {
			break
		}
		if cell.Ch == 0 {
			x--
		}
	}

	ok = true
	return
}

// CoordinatesDiff subtracts a from b.
func CoordinatesDiff(a, b term.Coordinates) term.Coordinates {
	return term.Coordinates{
		Y: a.Y - b.Y,
		X: a.X - b.X,
	}
}

// CoordinatesSum adds a to b.
func CoordinatesSum(a, b term.Coordinates) term.Coordinates {
	return term.Coordinates{
		Y: a.Y + b.Y,
		X: a.X + b.X,
	}
}

func nextWrite(c View) term.Coordinates {
	y := c.Rows() - 1
	x := len(c.RawCells()[y])
	return term.Coordinates{X: x, Y: y}
}
