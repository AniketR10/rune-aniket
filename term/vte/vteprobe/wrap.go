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
	"regexp"
	"strings"
)

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

// foldPlaceholderRE matches a fold placeholder anywhere in the body of
// a row. Editors typically render placeholders like
//
//	+--  5 lines: <preview>
//	+-- 12 lines folded -------
//
// We accept any "+- … <int> line(s)…" pattern as a fold marker without
// editor-specific tagging.
var foldPlaceholderRE = regexp.MustCompile(`\+-+\s*\d+\s+line[s]?\b`)

// detectWrap walks the content band [top, bot] and, given the
// rowToLine map produced by alignment, decides which rows are
// continuations (subsequent visual segments of a soft-wrapped line)
// and which are fold placeholders.
func detectWrap(
	rows []extractedRow,
	top, bot int,
	gutterWidth int,
	a alignment,
	lines []string,
) wrapInfo {
	totalRows := len(rows)
	w := wrapInfo{
		rowFileLine: make([]int, totalRows),
		wrapOffset:  make([]int, totalRows),
		folded:      make([]bool, totalRows),
	}

	prevLine := 0
	prevConsumed := 0
	for i := 0; i < len(a.rowToLine); i++ {
		y := top + i
		fileLine := a.rowToLine[i]
		body := stripGutter(rows[y].runes, gutterWidth)
		bodyStr := strings.TrimRight(string(body), " ")

		// Fold placeholder detection.
		if foldPlaceholderRE.MatchString(bodyStr) {
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
			expanded := ""
			if prevLine >= 1 && prevLine <= len(lines) {
				expanded = expandTabs(lines[prevLine-1], a.tabstop)
			}
			// Each continuation starts at the end of the previous
			// segment's visible content.
			prevConsumed += len(body)
			if prevConsumed > len(expanded) {
				prevConsumed = len(expanded)
			}
			w.wrapOffset[y] = prevConsumed - len(body)
			if w.wrapOffset[y] < 0 {
				w.wrapOffset[y] = 0
			}
			continue
		}

		w.rowFileLine[y] = fileLine
		w.wrapOffset[y] = 0
		prevLine = fileLine
		prevConsumed = len(body)
	}
	return w
}
