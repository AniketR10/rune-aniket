// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package cell

import (
	"math"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// Selection represents a selection of cells, defined by
// a from and to coordinates.
type Selection struct {
	From term.Coordinates
	To   term.Coordinates
}

// selector extends a view to perform cell selection operations.
type selector struct {
	view View
}

type iterateFunc func(int, term.Coordinates, term.Coordinates, []term.Cell)

func (s *selector) selectCells(from term.Coordinates, to term.Coordinates) (
	res [][]term.Cell, sels []Selection,
) {
	res = make([][]term.Cell, 0)
	s.iterateCells(from, to, func(i int, from, to term.Coordinates, cells []term.Cell) {
		res = append(res, cells)
		sels = append(sels, Selection{From: from, To: to})
	})
	return
}

func (s *selector) iterateCells(from term.Coordinates, to term.Coordinates, op iterateFunc) {
	from, to = term.CoordinatesSort(from, to)
	cells := s.view.RawCells()

	var i int
	for from.Y < to.Y && from.Y < len(cells) {
		x := int(math.Min(float64(from.X), float64(len(cells[from.Y]))))
		op(i,
			term.Coordinates{Y: from.Y, X: x},
			term.Coordinates{Y: from.Y, X: len(cells[from.Y])},
			cells[from.Y][x:])
		i++
		from.X = 0
		from.Y++
	}

	if from.Y >= len(cells) {
		return
	}

	fromX := int(math.Min(float64(from.X), float64(len(cells[from.Y]))))
	toX := int(math.Min(float64(to.X), float64(len(cells[from.Y]))))
	op(i,
		term.Coordinates{Y: from.Y, X: fromX},
		term.Coordinates{Y: from.Y, X: toX},
		cells[from.Y][fromX:toX],
	)
}

func (s *selector) selectLine(from term.Coordinates, to term.Coordinates) (
	res [][]term.Cell, sels []Selection,
) {
	res = make([][]term.Cell, 0)
	s.iterateLine(from, to, func(i int, from, to term.Coordinates, cells []term.Cell) {
		res = append(res, cells)
		sels = append(sels, Selection{From: from, To: to})
	})
	return
}

func (s *selector) iterateLine(from term.Coordinates, to term.Coordinates, op iterateFunc) {
	from, to = term.CoordinatesSort(from, to)
	cells := s.view.RawCells()

	i := 0
	for from.Y <= to.Y && from.Y < len(cells) {
		line := cells[from.Y][:]
		op(i,
			term.Coordinates{Y: from.Y, X: 0},
			term.Coordinates{Y: from.Y, X: len(line)},
			line,
		)
		i++
		from.Y++
	}
}

func (s *selector) iterateBlocks(
	from term.Coordinates, to term.Coordinates, op iterateFunc,
) {
	from, to = term.CoordinatesBlockSort(from, to)
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
	res [][]term.Cell, sels []Selection,
) {
	res = make([][]term.Cell, 0)
	s.iterateBlocks(from, to, func(i int, from, to term.Coordinates, cells []term.Cell) {
		res = append(res, cells)
		sels = append(sels, Selection{From: from, To: to})
	})
	return
}
