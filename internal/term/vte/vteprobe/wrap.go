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

package vteprobe

import "github.com/unstablebuild/rune-go-sdk/term"

// wrapInfo augments an alignment with per-row metadata about soft-wrap
// continuations and fold placeholders.
//
// rowFileLine[y] is the 1-based file line displayed on terminal row y
// (full row index, not band-relative). Rows outside the content band
// hold 0. wrapOffset[y] is the rune-cell offset into the file line at
// which row y starts (always 0 for the first segment of a wrapped line).
// folded[y] is true when the row is a fold placeholder.
type wrapInfo struct {
	rowFileLine []int
	wrapOffset  []int
	folded      []bool
}

func (w wrapInfo) fileLineAtRow(y int) (int, bool) {
	if y < 0 || y >= len(w.rowFileLine) {
		return 0, false
	}
	if w.rowFileLine[y] == 0 {
		return 0, false
	}
	return w.rowFileLine[y], true
}

func (w wrapInfo) foldedAtRow(y int) bool {
	if y < 0 || y >= len(w.folded) {
		return false
	}
	return w.folded[y]
}

func (w wrapInfo) wrapOffsetAtRow(y int) int {
	if y < 0 || y >= len(w.wrapOffset) {
		return 0
	}
	return w.wrapOffset[y]
}

// buildRowMappings projects a wrapInfo (indexed by full terminal row)
// into the public RowMapping shape exported on Result.
func buildRowMappings(totalRows int, w wrapInfo) []RowMapping {
	out := make([]RowMapping, totalRows)
	for y := 0; y < totalRows; y++ {
		if y < len(w.rowFileLine) {
			out[y].FileLine = w.rowFileLine[y]
		}
		if y < len(w.wrapOffset) {
			out[y].WrapOffset = w.wrapOffset[y]
		}
		if y < len(w.folded) {
			out[y].Folded = w.folded[y]
		}
	}
	return out
}

// detectWrap walks the content band [top, bot] and, given the
// rowToLine map produced by alignment, decides which rows are
// continuations (subsequent visual segments of a soft-wrapped line)
// and which are fold placeholders.
func detectWrap(
	rows []extractedRow,
	top int,
	gutterWidth int,
	a alignment,
	lines [][]term.Cell,
	slab *Slab,
) wrapInfo {
	totalRows := len(rows)
	w := slab.wrapInfoBuf(totalRows)
	var expanded [][]rune

	prevLine := 0
	prevConsumed := 0
	for i := 0; i < len(a.rowToLine); i++ {
		y := top + i
		fileLine := a.rowToLine[i]
		body := stripGutter(rows[y].runes, gutterWidth)
		bodyWidth := len(body)
		trimmedBody := trimRightSpace(body)

		// Fold placeholder detection.
		if isFoldPlaceholderBody(trimmedBody) {
			w.rowFileLine[y] = fileLine
			w.folded[y] = true
			prevLine = fileLine
			prevConsumed = 0
			continue
		}

		// If alignment did not anchor this row to a file line, treat it
		// as a continuation of the previous file line (soft wrap).
		if fileLine == 0 && prevLine > 0 {
			w.rowFileLine[y] = prevLine
			expandedLen := 0
			if prevLine >= 1 && prevLine <= len(lines) {
				if expanded == nil {
					expanded = slab.expandLinesFor(lines, a.tabstop)
				}
				expandedLen = len(expanded[prevLine-1])
			}
			// Each continuation starts at the end of the previous
			// segment's visible content.
			prevConsumed += bodyWidth
			if prevConsumed > expandedLen {
				prevConsumed = expandedLen
			}
			w.wrapOffset[y] = prevConsumed - bodyWidth
			if w.wrapOffset[y] < 0 {
				w.wrapOffset[y] = 0
			}
			continue
		}

		w.rowFileLine[y] = fileLine
		w.wrapOffset[y] = 0
		prevLine = fileLine
		prevConsumed = bodyWidth
	}
	return w
}

// isFoldPlaceholderBody recognises editor fold markers like
// "+--  5 lines: preview" without converting every rendered row to a
// string for the common non-fold path.
func isFoldPlaceholderBody(body []rune) bool {
	for i := 0; i < len(body); i++ {
		if body[i] != '+' {
			continue
		}
		j := i + 1
		if j >= len(body) || body[j] != '-' {
			continue
		}
		for j < len(body) && body[j] == '-' {
			j++
		}
		for j < len(body) && body[j] == ' ' {
			j++
		}
		startDigits := j
		for j < len(body) && body[j] >= '0' && body[j] <= '9' {
			j++
		}
		if j == startDigits {
			continue
		}
		if j >= len(body) || body[j] != ' ' {
			continue
		}
		for j < len(body) && body[j] == ' ' {
			j++
		}
		if hasWordAt(body[j:], []rune("line")) {
			return true
		}
	}
	return false
}

func hasWordAt(body, word []rune) bool {
	if len(body) < len(word) {
		return false
	}
	for i, r := range word {
		if body[i] != r {
			return false
		}
	}
	end := len(word)
	if end < len(body) && body[end] == 's' {
		end++
	}
	return end == len(body) || !isWordRune(body[end])
}

func isWordRune(r rune) bool {
	return r == '_' || r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z'
}
