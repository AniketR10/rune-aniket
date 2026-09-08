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

package workspace

import (
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/cell"
)

// NewUnixFileView returns newly initialized UnixFileView.
func NewUnixFileView(v cell.View) UnixFileView {
	return UnixFileView{view: v}
}

// UnixFileView is a cell.View that hides the last EOL if present,
// to account for unix last EOL termination.
type UnixFileView struct {
	view cell.View
}

// EndsWithEOL returns true if the underlying cell.View ends
// with a new line.
func (b UnixFileView) EndsWithEOL() bool {
	cells := b.view.RawCells()
	return len(cells) > 1 && len(cells[len(cells)-1]) == 0
}

// Rows satisfies cell.View.
func (b UnixFileView) Rows() (rows int) {
	rows = b.view.Rows()
	if !b.EndsWithEOL() {
		return
	}
	rows--
	return

}

// Columns satisfies cell.View.
func (b UnixFileView) Columns(row int) int {
	return b.view.Columns(row)
}

// Cell satisfies cell.View.
func (b UnixFileView) Cell(pos term.Coordinates) (term.Cell, bool) {
	return b.view.Cell(pos)
}

// RawCells satisfies cell.View.
func (b UnixFileView) RawCells() (cells [][]term.Cell) {
	cells = b.view.RawCells()
	if !b.EndsWithEOL() {
		return
	}
	cells = cells[:len(cells)-1]
	return
}

// String satisfies cell.View.
func (b UnixFileView) String() string {
	if !b.EndsWithEOL() {
		return b.view.String()
	}
	return term.CellsToString(b.RawCells())
}
