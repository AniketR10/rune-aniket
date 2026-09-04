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

package vteprobe

import (
	"github.com/unstablebuild/rune-go-sdk/term"
)

// extractedRow is a single rendered row from the cell grid, normalized
// for easier downstream processing.
//
// runes contains one rune per visible visual cell (wide cells contribute
// the leading rune and any combining marks via combining, NOT a copy of
// the leading rune per visual cell). Empty / zero-rune cells are
// represented as spaces.
//
// attrs is aligned with runes: attrs[i] is the attribute mask of the
// cell that produced runes[i].
//
// runeColAt maps a visual column x (the X used by term.Coordinates) to
// the index into runes. It accounts for wide characters that span two
// visual cells.
type extractedRow struct {
	runes      []rune
	attrs      []term.Attributes
	runeColMap []int // visualX -> index into runes, length is visual width
}

func (r *extractedRow) runeColAt(visualX int) int {
	if visualX < 0 {
		return 0
	}
	if visualX >= len(r.runeColMap) {
		return len(r.runes)
	}
	return r.runeColMap[visualX]
}

// extractRows extracts each row of the rendered cell grid into an
// extractedRow, trimming trailing space-padded cells while preserving
// columns that contain real content. The input is the raw cell grid
// (typically obtained via Scroll.Buffer().RawCells() on a vte.Component
// or the cloned grid returned by vte.Replay), so callers do not need
// to wrap it in a cell.Buffer first.
func extractRowsWithSlab(raw [][]term.Cell, slab *Slab) []extractedRow {
	rows, _ := slab.beginExtractRows(raw)
	runeOff := 0
	colOff := 0
	for y, row := range raw {
		startRune := runeOff
		startCol := colOff
		idx := 0
		for _, c := range row {
			width := int(c.Width)
			if width < 1 {
				width = 1
			}
			ch := c.Ch
			if ch == 0 {
				ch = ' '
			}
			slab.rowRune[runeOff] = ch
			slab.rowAttr[runeOff] = c.Attributes()
			runeOff++
			for k := 0; k < width; k++ {
				slab.rowCol[colOff] = idx
				colOff++
			}
			idx++
		}
		rows[y] = extractedRow{
			runes:      slab.rowRune[startRune:runeOff:runeOff],
			attrs:      slab.rowAttr[startRune:runeOff:runeOff],
			runeColMap: slab.rowCol[startCol:colOff:colOff],
		}
	}
	return rows
}

// gridWidth reports the visual width of the rendered grid, taken from
// the widest row. Returns 0 for an empty grid.
func gridWidth(rows []extractedRow) int {
	w := 0
	for i := range rows {
		if n := len(rows[i].runeColMap); n > w {
			w = n
		}
	}
	return w
}
