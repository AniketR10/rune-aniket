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

// gutter describes the line-number gutter on the left of the content
// band. width is the visual column width occupied by the gutter
// (including the trailing separator space). lineNo[i] is the parsed line
// number for content row top+i, or 0 when the row had no parsable
// number (continuation rows, relative-zero rows, etc.).
type gutter struct {
	present bool
	width   int
	lineNo  []int // indexed [0, bot-top]; 0 means "no parsable number"
}

// detectGutter scans rows in the content band [top, bot] for a
// left-aligned line-number gutter.
//
// A row contributes to the gutter when its leftmost non-space cluster is
// a numeric token followed by at least one space (or a vertical-bar
// separator). The gutter is considered present when at least half of
// the rows in the band contribute a parseable number AND there is at
// least one strictly increasing pair (so a column of zeros is not
// mistaken for a gutter).
func detectGutter(rows []extractedRow, top, bot int) gutter {
	if top > bot || top < 0 || bot >= len(rows) {
		return gutter{}
	}

	// Find candidate widths per row.
	type rowGutter struct {
		number int
		width  int // visual width of the gutter on this row
	}
	band := bot - top + 1
	parsed := make([]rowGutter, band)
	contributing := 0
	for i := 0; i < band; i++ {
		r := rows[top+i]
		n, w := parseLeadingLineNumber(r.runes)
		parsed[i] = rowGutter{number: n, width: w}
		if w > 0 {
			contributing++
		}
	}
	if contributing*2 < band {
		return gutter{}
	}

	// Find the most common positive width — that's the gutter width.
	counts := map[int]int{}
	for _, p := range parsed {
		if p.width > 0 {
			counts[p.width]++
		}
	}
	bestWidth, bestCount := 0, 0
	for w, c := range counts {
		if c > bestCount || (c == bestCount && w > bestWidth) {
			bestWidth, bestCount = w, c
		}
	}
	if bestWidth == 0 {
		return gutter{}
	}

	// Build the lineNo slice; widen mismatched rows up to bestWidth by
	// rejecting them (lineNo=0) but keep the gutter width consistent so
	// alignment can still strip a fixed prefix. We require at least one
	// positive line number across the band — a column of pure zeros is
	// not a gutter.
	// Accept any row whose width is >= bestWidth as a gutter row: the
	// extra columns are body indentation that we leave for the content
	// stripper to handle. Rows that did not parse a line number at all
	// (or whose width is somehow smaller) are gaps.
	lineNo := make([]int, band)
	sawPositive := false
	for i, p := range parsed {
		if p.width < bestWidth {
			lineNo[i] = 0
			continue
		}
		lineNo[i] = p.number
		if p.number > 0 {
			sawPositive = true
		}
	}
	if !sawPositive {
		return gutter{}
	}

	return gutter{present: true, width: bestWidth, lineNo: lineNo}
}

// parseLeadingLineNumber parses a leading line-number token at the start
// of runes and returns the parsed number plus the visual width consumed
// (leading spaces + digit runes + separator). The separator is one
// space, optionally followed by a vertical-bar (`|` or `│`) and a
// space. We deliberately do NOT keep consuming whitespace beyond the
// separator because the next runs of spaces belong to the file
// content's indentation. When the row does not start with a numeric
// token, parseLeadingLineNumber returns (0, 0).
func parseLeadingLineNumber(runes []rune) (number int, width int) {
	// Skip leading spaces (right-aligned numbers are common).
	i := 0
	for i < len(runes) && runes[i] == ' ' {
		i++
	}
	digitsStart := i
	for i < len(runes) && runes[i] >= '0' && runes[i] <= '9' {
		number = number*10 + int(runes[i]-'0')
		i++
	}
	if i == digitsStart {
		return 0, 0
	}
	// Require a space immediately after the digits.
	if i >= len(runes) || runes[i] != ' ' {
		return 0, 0
	}
	// Consume any number of spaces after the digits (some editors
	// render two spaces of padding before the body content). We then
	// optionally consume a single vertical-bar separator followed by
	// more spaces (helix-style ` NN │ `).
	for i < len(runes) && runes[i] == ' ' {
		i++
	}
	if i < len(runes) && (runes[i] == '|' || runes[i] == '│') {
		i++
		for i < len(runes) && runes[i] == ' ' {
			i++
		}
	}
	return number, i
}
