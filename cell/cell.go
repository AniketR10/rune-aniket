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

package cell

import (
	"context"
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// View is the interface that wraps methods to query a 2D matrix of term.Cell.
type View interface {
	Rows() int
	Columns(row int) int
	Cell(term.Coordinates) (term.Cell, bool)
	RawCells() [][]term.Cell
	fmt.Stringer
}

// Editor is the interface that wraps methods to mutate a 2D matrix of term.Cell.
type Editor interface {
	// Edit replaces any content from [start:end) with str and returns the
	// right-exclusive coordinates of the effective insert range. Note that
	// returned from, to values will be equal to each other if this operation
	// only removes content. This effectively allows clients to reverse a call
	// to Edit by calling it again with the last return values.
	//
	// This method should panic if delete range between start, end is out of bounds.
	Edit(ctx context.Context, start, end term.Coordinates, new string) (
		from, to term.Coordinates, old string,
	)
}

// NewView returns a new Reader which reads from cells and uses tabspaces.
func NewView(cells [][]term.Cell) View {
	r := &rawCells{
		cells:     cells,
		columnCap: defColumnCap,
		rowCap:    defRowCap,
	}
	r.setFillInChar(' ')
	return r
}
