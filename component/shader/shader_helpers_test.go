// Copyright (C) 2017-2026 Unstable Build, LLC
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

package shader_test

import (
	"github.com/unstablebuild/rune-go-sdk/term"
)

func makeCharCells(cols, rows int) [][]term.Cell {
	out := make([][]term.Cell, rows)
	for y := range rows {
		out[y] = make([]term.Cell, cols)
		for x := range cols {
			out[y][x] = term.NewCell('A', 1,
				term.Attributes{Fg: term.ColorWhite, Bg: term.ColorBlack})
		}
	}
	return out
}

func makeBlankCells(cols, rows int) [][]term.Cell {
	out := make([][]term.Cell, rows)
	for y := range rows {
		out[y] = make([]term.Cell, cols)
		for x := range cols {
			out[y][x] = term.NewCell(' ', 1,
				term.Attributes{Fg: term.ColorWhite, Bg: term.ColorBlack})
		}
	}
	return out
}

func cloneCells(in [][]term.Cell) [][]term.Cell {
	out := make([][]term.Cell, len(in))
	for y, row := range in {
		out[y] = make([]term.Cell, len(row))
		copy(out[y], row)
	}
	return out
}
