// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.
package cell

import (
	"strings"

	"unstable.build/go-tui/term"
)

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
	_, _ = builder.ReadFrom(strings.NewReader(str))
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
	ret.initWithCells(cells)
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

	// lines ending in null, could derail assumptions below
	// better to just abort conversion of nulls
	lastSane := term.Coordinates{}
	for xi, c := range line {
		if c.Ch == 0 {
			ret.X++
			if xi == len(line)-1 {
				ret = lastSane
			}
		} else {
			lastSane = ret
		}
		if xi == ret.X {
			break
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

func nextWrite(c View) term.Coordinates {
	y := c.Rows() - 1
	x := len(c.RawCells()[y])
	return term.Coordinates{X: x, Y: y}
}
