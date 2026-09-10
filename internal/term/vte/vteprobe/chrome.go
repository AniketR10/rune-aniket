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

import (
	"unicode"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// detectChrome finds the inclusive content band [top, bot] inside rows
// by peeling chrome off the top and bottom edges.
//
// fileLines is consulted when deciding whether a non-attributed bottom
// row is chrome: rows whose text appears verbatim in the file are kept
// as content; rows whose text does not appear are peeled.
//
// Heuristics are purely structural:
//
//   - Top edge: peel up to three attribute-marked rows (tab bar, title
//     bar, separator).
//
//   - Bottom edge: peel as long as the row is blank, attribute-marked,
//     or a short non-content message (up to two consecutive such soft
//     messages — covers cases like a status row followed by a one-shot
//     editor message ("Loaded 1 file.")).
//
// The band always retains at least one content row. Whether the cursor
// lies in chrome is the caller's responsibility.
func detectChrome(rows []extractedRow, fileLines [][]term.Cell) (top, bot int) {
	if len(rows) == 0 {
		return 0, -1
	}
	top, bot = 0, len(rows)-1

	peelTop(rows, &top, bot, fileLines)
	peelBottom(rows, &bot, top, fileLines)
	return top, bot
}

// peelTop walks top upward, peeling rows that are blank,
// attribute-marked, or short messages that do not appear in the file
// (e.g. nano's "File: <path>" title bar). We tolerate up to two such
// soft messages before stopping. When fileLines is nil/empty we fall
// back to attribute-only chrome detection to avoid eating content rows
// in unit-test scenarios that intentionally skip the file context.
//
// A leading blank row is padding (a title/content separator) and is
// peeled, unless it mirrors a real blank file line at the top of the
// viewport: when the content just below the blank matches a file line
// that is itself preceded by a blank line, the rendered blank is that
// preceding file line, so peeling it would drop a line and shift the
// inferred scroll position.
func peelTop(rows []extractedRow, top *int, bot int, fileLines [][]term.Cell) {
	softInARow := 0
	peeledChrome := false
	for *top < bot {
		row := rows[*top]
		text := trimRightSpace(row.runes)
		if len(text) == 0 {
			if !peeledChrome || leadingBlankIsFileLine(rows, *top, bot, fileLines) {
				return
			}
			*top++
			softInARow = 0
			continue
		}
		// Editors that highlight the matching-bracket line or
		// otherwise paint a background colour on a content row look
		// chrome-y to isAttrChromeRow even though their text is
		// straight from the file. When the file is available, keep
		// any row whose text appears in it before considering
		// attribute-based peeling.
		if len(fileLines) > 0 && appearsInFile(text, fileLines) {
			return
		}
		if isAttrChromeRow(row) {
			*top++
			softInARow = 0
			peeledChrome = true
			continue
		}
		if len(fileLines) > 0 && softInARow < 2 && !appearsInFile(text, fileLines) {
			*top++
			softInARow++
			peeledChrome = true
			continue
		}
		return
	}
}

// peelBottom walks bot downward, peeling chrome rows. A row counts as
// chrome when it is blank, attribute-marked, or its text does not
// appear in fileLines. When fileLines is nil/empty we restrict the
// soft path so unit tests calling with nil keep the conservative
// behaviour (attribute-marked and blank rows only).
func peelBottom(rows []extractedRow, bot *int, top int, fileLines [][]term.Cell) {
	softInARow := 0
	for *bot > top {
		row := rows[*bot]
		text := trimRightSpace(row.runes)
		if len(text) == 0 {
			*bot--
			softInARow = 0
			continue
		}
		if len(fileLines) > 0 && appearsInFile(text, fileLines) {
			return
		}
		if isAttrChromeRow(row) {
			*bot--
			softInARow = 0
			continue
		}
		if len(fileLines) > 0 && softInARow < 2 && !appearsInFile(text, fileLines) {
			*bot--
			softInARow++
			continue
		}
		return
	}
}

// appearsInFile returns true when text is a prefix or suffix of any
// file line (which is enough for our purposes — a row containing a
// trimmed-down version of a file line is content, not chrome).
func appearsInFile(text []rune, fileLines [][]term.Cell) bool {
	trimmed := trimSpaceRunes(text)
	if len(trimmed) == 0 {
		return false
	}
	for _, line := range fileLines {
		trimmedLine := trimSpaceCells(line)
		if len(trimmedLine) == 0 {
			continue
		}
		if containsCellsRunes(trimmedLine, trimmed) || containsRunesCells(trimmed, trimmedLine) {
			return true
		}
	}
	return false
}

func trimSpaceRunes(rs []rune) []rune {
	start, end := 0, len(rs)
	for start < end && unicode.IsSpace(rs[start]) {
		start++
	}
	for end > start && unicode.IsSpace(rs[end-1]) {
		end--
	}
	return rs[start:end]
}

func trimSpaceCells(cells []term.Cell) []term.Cell {
	start, end := 0, len(cells)
	for start < end && unicode.IsSpace(cells[start].Ch) {
		start++
	}
	for end > start && unicode.IsSpace(cells[end-1].Ch) {
		end--
	}
	return cells[start:end]
}

func containsCellsRunes(hay []term.Cell, needle []rune) bool {
	if len(needle) == 0 {
		return true
	}
	if len(hay) < len(needle) {
		return false
	}
	for i := 0; i <= len(hay)-len(needle); i++ {
		ok := true
		for j, r := range needle {
			if hay[i+j].Ch != r {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

func containsRunesCells(hay []rune, needle []term.Cell) bool {
	if len(needle) == 0 {
		return true
	}
	if len(hay) < len(needle) {
		return false
	}
	for i := 0; i <= len(hay)-len(needle); i++ {
		ok := true
		for j, cell := range needle {
			if hay[i+j] != cell.Ch {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// isAttrChromeRow returns true when at least 70% of the row's non-blank
// cells carry chrome-looking attributes.
func isAttrChromeRow(r extractedRow) bool {
	if len(r.runes) == 0 {
		return false
	}
	visible, chrome := 0, 0
	for i, ru := range r.runes {
		if ru == ' ' || ru == 0 {
			continue
		}
		visible++
		if isChromeAttr(r.attrs[i]) {
			chrome++
		}
	}
	if visible == 0 {
		return false
	}
	return float64(chrome)/float64(visible) >= 0.7
}

// leadingBlankIsFileLine reports whether the blank row at blankIdx is a
// real blank file line shown at the top of the viewport rather than a
// title/content separator. It locates the first non-blank content row
// below the blank in the file (disambiguated by also matching the row
// after it to the next file line) and returns true when that file line
// is itself preceded by a blank line — i.e. the rendered blank mirrors
// fileLines[k-1].
func leadingBlankIsFileLine(rows []extractedRow, blankIdx, bot int, fileLines [][]term.Cell) bool {
	if len(fileLines) == 0 {
		return false
	}
	first := blankIdx + 1
	for first <= bot && len(trimRightSpace(rows[first].runes)) == 0 {
		first++
	}
	if first > bot {
		return false
	}
	firstText := trimSpaceRunes(rows[first].runes)
	if len(firstText) == 0 {
		return false
	}
	var nextText []rune
	if first+1 <= bot {
		nextText = trimSpaceRunes(rows[first+1].runes)
	}
	// k is a 0-based file line index. A leading blank can only be a real
	// file line when the content below maps to file line >= 2 (so a
	// preceding line exists).
	for k := 1; k < len(fileLines); k++ {
		line := trimSpaceCells(fileLines[k])
		if !runesEqualCells(firstText, line) {
			continue
		}
		if len(nextText) > 0 && k+1 < len(fileLines) {
			if !runesEqualCells(nextText, trimSpaceCells(fileLines[k+1])) {
				continue
			}
		}
		if len(trimSpaceCells(fileLines[k-1])) == 0 {
			return true
		}
	}
	return false
}

func runesEqualCells(rs []rune, cells []term.Cell) bool {
	if len(rs) != len(cells) {
		return false
	}
	for i := range rs {
		if rs[i] != cells[i].Ch {
			return false
		}
	}
	return true
}

// isChromeAttr returns true when the attribute mask of a cell signals
// chrome (reverse video, bold-on-color, or non-default background).
const colorReset = term.ColorSpecial

func isChromeAttr(a term.Attributes) bool {
	s := a.Style()
	if s.Attrs&term.AttrReverse != 0 {
		return true
	}
	if s.Attrs&term.AttrBold != 0 {
		if s.Fg != term.ColorDefault && s.Bg != term.ColorDefault {
			return true
		}
	}
	if s.Bg != term.ColorDefault && s.Bg != colorReset {
		return true
	}
	return false
}
