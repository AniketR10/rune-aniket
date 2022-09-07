package cell

import (
	"math"

	"github.com/ernestrc/go-tui/term"
)

// selector extends a view to perform cell selection operations.
type selector struct {
	view View
}

func (s *selector) selectCells(from term.Coordinates, to term.Coordinates) (
	res [][]term.Cell,
) {
	res = make([][]term.Cell, 0)
	from, to = SortFromTo(from, to)
	cells := s.view.RawCells()

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
	toX := int(math.Min(float64(to.X), float64(len(cells[from.Y]))))
	res = append(res, cells[from.Y][fromX:toX])

	return
}

func (s *selector) selectLine(from term.Coordinates, to term.Coordinates) (
	res [][]term.Cell,
) {
	res = make([][]term.Cell, 0)
	from, to = SortFromTo(from, to)
	cells := s.view.RawCells()

	for from.Y <= to.Y && from.Y < len(cells) {
		res = append(res, cells[from.Y][:])
		from.Y++
	}

	res = append(res, make([]term.Cell, 0))

	return
}

type iterateBlockFunc func(int, term.Coordinates, term.Coordinates, []term.Cell)

func (s *selector) iterateBlocks(
	from term.Coordinates, to term.Coordinates, op iterateBlockFunc,
) {
	from, to = SortFromToBlock(from, to)
	cells := s.view.RawCells()
	i := 0
	lencells := len(cells)
	for from.Y <= to.Y && from.Y < lencells {
		maxx := float64(len(cells[from.Y]))
		xfrom := int(math.Min(maxx, float64(from.X)))
		xto := int(math.Min(maxx, float64(to.X)))

		op(i,
			term.Coordinates{Y: from.Y, X: xfrom},
			term.Coordinates{Y: from.Y, X: xto},
			cells[from.Y][xfrom:xto])

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
