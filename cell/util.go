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

func findTabspaces(in [][]term.Cell) int {
	var zeroRunes int
	for _, r := range in {
		for _, c := range r {
			if c.Ch == 0 {
				zeroRunes++
				continue
			}

			if zeroRunes == 0 {
				continue
			}

			if c.Ch == '\t' {
				return zeroRunes + 1
			}

			zeroRunes = 0
		}
	}

	return 1
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
func CellsToBuffer(c [][]term.Cell) *Buffer {
	cells := new(rawCells)

	tabspaces := findTabspaces(c)
	cells.init(tabspaces)

	cells.cells = CloneCells(c)

	ret := new(Buffer)
	ret.initWithCells(cells, false)
	return ret
}

// ConvertRuneCoordinates converts x and y, which use the buffer runes as offsets
// into term.Coordinates, which account for tab expansion. It returns false if y is out
// of bounds.
func ConvertRuneCoordinates(cells [][]term.Cell, y, x int) (term.Coordinates, bool) {
	if y >= len(cells) {
		return term.Coordinates{}, false
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
	return term.Coordinates{Y: y, X: x}, true
}

// ConvertTermCoordinates converts c, which use the terminal system of coordinates, which
// account for tab expansion, into rune offsets. It returns false if y is out
// of bounds.
func ConvertTermCoordinates(cells [][]term.Cell, c term.Coordinates) (y, x int, ok bool) {
	if c.Y >= len(cells) {
		return
	}

	line := cells[c.Y]
	y = c.Y
	x = c.X

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
