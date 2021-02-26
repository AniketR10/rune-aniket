package cell

import (
	"math"

	"github.com/ernestrc/go-tui/term"
)

// selector extends a reader to perform cell selection operations.
type selector struct {
	reader Reader
}

func (s *selector) selectCells(from term.Coordinates, to term.Coordinates) (
	res [][]term.Cell,
) {
	res = make([][]term.Cell, 0)
	from, to = SortFromTo(from, to)
	cells := s.reader.RawCells()

	for from.Y < to.Y && from.Y < len(cells) {
		x := int(math.Min(float64(from.X), float64(len(cells[from.Y]))))
		res = append(res, cells[from.Y][x:])
		from.X = 0
		from.Y++
	}

	if from.Y >= len(cells) {
		return
	}

	fromX := int(math.Min(float64(from.X), float64(len(cells[from.Y]))))
	toX := int(math.Min(float64(to.X+1), float64(len(cells[from.Y]))))
	res = append(res, cells[from.Y][fromX:toX])

	if to.X == len(cells[from.Y]) && to.Y < len(cells)-1 {
		res = append(res, make([]term.Cell, 0))
	}
	return
}

func (s *selector) selectLine(from term.Coordinates, to term.Coordinates) (
	res [][]term.Cell,
) {
	res = make([][]term.Cell, 0)
	from, to = SortFromTo(from, to)
	cells := s.reader.RawCells()

	for from.Y <= to.Y && from.Y < len(cells) {
		res = append(res, cells[from.Y][:])
		from.Y++
	}

	if from.Y < len(cells)-1 {
		res = append(res, make([]term.Cell, 0))
	}

	return
}

type iterateBlockFunc func(int, term.Coordinates, term.Coordinates, []term.Cell)

func (s *selector) iterateBlocks(
	from term.Coordinates, to term.Coordinates, op iterateBlockFunc,
) {
	from, to = SortFromToBlock(from, to)
	cells := s.reader.RawCells()
	i := 0
	for from.Y <= to.Y && from.Y < len(cells) {
		maxy := float64(len(cells[from.Y]))
		xfrom := int(math.Min(maxy, float64(from.X)))
		xtolen := int(math.Min(maxy, float64(to.X+1)))
		xto := int(math.Min(maxy, float64(to.X)))

		op(i,
			term.Coordinates{Y: from.Y, X: xfrom},
			term.Coordinates{Y: from.Y, X: xto},
			cells[from.Y][xfrom:xtolen])

		from.Y++
		i++
	}
}

func (s *selector) selectBlock(from term.Coordinates, to term.Coordinates) (
	res [][]term.Cell,
) {
	res = make([][]term.Cell, 0)
	s.iterateBlocks(from, to, func(i int, from, to term.Coordinates, cells []term.Cell) {
		res = append(res, cells)
	})
	return
}
