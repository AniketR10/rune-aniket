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
func StringToCells(str string) (cells [][]term.Cell) {
	var builder rawCells
	builder.init(defTabSpaces)
	builder.ReadFrom(strings.NewReader(str))
	return builder.RawCells()
}

// ConvertCoordinates converts x and y, which use the buffer in bytes as offsets
// into term.Coordinates, which account for tab expansion.
func ConvertCoordinates(cells [][]term.Cell, y, x int) term.Coordinates {
	if y >= len(cells) {
		return term.Coordinates{}
	}
	line := cells[y]

	for xi, c := range line {
		if xi == x {
			break
		}
		if c.Ch == 0 {
			x++
		}
	}
	return term.Coordinates{Y: y, X: x}
}
