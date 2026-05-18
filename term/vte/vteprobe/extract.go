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

// text returns the rendered row as a string, useful for regex matching.
func (r *extractedRow) text() string {
	return string(r.runes)
}

// extractRows extracts each row of the rendered cell grid into an
// extractedRow, trimming trailing space-padded cells while preserving
// columns that contain real content. The input is the raw cell grid
// (typically obtained via Scroll.Buffer().RawCells() on a vte.Component
// or the cloned grid returned by vte.Replay), so callers do not need
// to wrap it in a cell.Buffer first.
func extractRows(raw [][]term.Cell) []extractedRow {
	out := make([]extractedRow, len(raw))
	for y, row := range raw {
		out[y] = extractRow(row)
	}
	return out
}

func extractRow(cells []term.Cell) extractedRow {
	runes := make([]rune, 0, len(cells))
	attrs := make([]term.Attributes, 0, len(cells))
	runeCol := make([]int, 0, len(cells))
	idx := 0
	for _, c := range cells {
		width := int(c.Width)
		if width < 1 {
			width = 1
		}
		ch := c.Ch
		if ch == 0 {
			ch = ' '
		}
		runes = append(runes, ch)
		attrs = append(attrs, c.Attributes)
		// The leading visual cell of a wide rune maps to idx; subsequent
		// continuation cells map to the same idx so that a cursor placed
		// over either cell of a wide rune resolves to the same rune.
		for k := 0; k < width; k++ {
			runeCol = append(runeCol, idx)
		}
		idx++
	}
	return extractedRow{runes: runes, attrs: attrs, runeColMap: runeCol}
}
