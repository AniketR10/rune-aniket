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
	"strings"

	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
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
func detectChrome(rows []extractedRow, fileLines []string) (top, bot int) {
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
func peelTop(rows []extractedRow, top *int, bot int, fileLines []string) {
	softInARow := 0
	for *top < bot {
		row := rows[*top]
		text := strings.TrimRight(row.text(), " ")
		if text == "" {
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
			continue
		}
		if len(fileLines) > 0 && softInARow < 2 && !appearsInFile(text, fileLines) {
			*top++
			softInARow++
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
func peelBottom(rows []extractedRow, bot *int, top int, fileLines []string) {
	softInARow := 0
	for *bot > top {
		row := rows[*bot]
		text := strings.TrimRight(row.text(), " ")
		if text == "" {
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
func appearsInFile(text string, fileLines []string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	for _, line := range fileLines {
		ts := strings.TrimSpace(line)
		if ts == "" {
			continue
		}
		if strings.Contains(ts, trimmed) || strings.Contains(trimmed, ts) {
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

// isChromeAttr returns true when the attribute mask of a cell signals
// chrome (reverse video, bold-on-color, or non-default background).
func isChromeAttr(a term.Attributes) bool {
	s := tcell.Style(a)
	if s.Attrs&tcell.AttrReverse != 0 {
		return true
	}
	if s.Attrs&tcell.AttrBold != 0 {
		if s.Fg != tcell.ColorDefault && s.Bg != tcell.ColorDefault {
			return true
		}
	}
	if s.Bg != tcell.ColorDefault && s.Bg != tcell.ColorReset {
		return true
	}
	return false
}
