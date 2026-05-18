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
)

// alignment is the result of mapping rendered rows in the content band
// back to file lines.
//
// rowToLine[i] is the 1-based file line displayed on row top+i, or 0 if
// the row could not be matched (e.g. trailing empty rows below EOF).
// topFileLine is the file line on the first row of the band.
// tabstop is the candidate tabstop that produced the best fit.
// coverage is the fraction of content rows in the band that matched
// their corresponding file line cleanly.
type alignment struct {
	ok          bool
	rowToLine   []int
	topFileLine int
	tabstop     int
	coverage    float64
}

// alignByGutter aligns rows using a gutter that has already been
// detected. Rows with a positive lineNo are anchored to that exact file
// line; rows without a parsed number inherit (prev + 1).
//
// alignByGutter picks the tabstop with the best agreement between
// rendered body (after the gutter) and the corresponding file line.
func alignByGutter(
	rows []extractedRow,
	top, bot int,
	gut gutter,
	lines []string,
	tabstopHints []int,
) alignment {
	band := bot - top + 1
	if band <= 0 || len(gut.lineNo) != band {
		return alignment{}
	}

	// Derive rowToLine from gutter numbers, filling gaps with prev+1.
	rowToLine := make([]int, band)
	topFileLine := 0
	prev := 0
	for i := 0; i < band; i++ {
		n := gut.lineNo[i]
		switch {
		case n > 0:
			rowToLine[i] = n
			prev = n
			if topFileLine == 0 {
				topFileLine = n
			}
		case prev > 0:
			rowToLine[i] = prev + 1
			prev++
		default:
			rowToLine[i] = 0
		}
	}
	if topFileLine == 0 {
		return alignment{}
	}

	// Score each tabstop and pick the best.
	bestTab, bestScore := tabstopHints[0], -1.0
	for _, ts := range tabstopHints {
		score := scoreAlignment(rows, top, gut.width, rowToLine, lines, ts)
		if score > bestScore {
			bestScore = score
			bestTab = ts
		}
	}
	if bestScore < 0.6 {
		return alignment{}
	}
	return alignment{
		ok:          true,
		rowToLine:   rowToLine,
		topFileLine: topFileLine,
		tabstop:     bestTab,
		coverage:    bestScore,
	}
}

// alignByContentWithGutter aligns rows without trusting any gutter
// numbers by greedy content matching: it picks the first non-empty
// row's body (after stripping gutterWidth runes), finds the file line
// whose tab-expanded rendering best matches it, and assumes consecutive
// rows map to consecutive file lines starting from that anchor.
//
// Use gutterWidth == 0 when there is no gutter; pass the detected
// gutter width when gutter numbers are present but cannot be trusted
// (relative numbering, for example).
func alignByContentWithGutter(
	rows []extractedRow,
	top, bot int,
	gutterWidth int,
	lines []string,
	tabstopHints []int,
) alignment {
	band := bot - top + 1
	if band <= 0 {
		return alignment{}
	}

	bestTab := tabstopHints[0]
	var best alignment
	for _, ts := range tabstopHints {
		a := alignByContentForTabstop(rows, top, bot, gutterWidth, lines, ts)
		// When the simple 1:1 alignment underperforms, try a
		// wrap-aware variant that lets a single file line span
		// multiple consecutive rows.
		if !a.ok || a.coverage < 0.95 {
			b := alignWithWrap(rows, top, bot, gutterWidth, lines, ts)
			if b.ok && b.coverage > a.coverage {
				a = b
			}
		}
		if a.ok && a.coverage > best.coverage {
			best = a
			bestTab = ts
		}
	}
	if !best.ok || best.coverage < 0.6 {
		return alignment{}
	}
	best.tabstop = bestTab
	return best
}

// alignWithWrap assigns rows to file lines under the assumption that a
// single file line may visually span multiple consecutive rows.
//
// The algorithm picks a starting anchor file line whose expanded
// rendering matches the first non-empty row's prefix, then walks rows
// and file lines together: each file line consumes as many rows as its
// expanded width requires (rounded up against the visible body width
// of the band).
func alignWithWrap(
	rows []extractedRow,
	top, bot int,
	gutterWidth int,
	lines []string,
	tabstop int,
) alignment {
	band := bot - top + 1
	if band <= 0 {
		return alignment{}
	}
	// Estimate terminal body width from the first row.
	bodyWidth := 0
	if len(rows) > 0 {
		bodyWidth = len(rows[top].runeColMap) - gutterWidth
	}
	if bodyWidth <= 0 {
		return alignment{}
	}

	// Anchor selection: find the file line whose expanded text begins
	// with the first non-empty row body. We try every candidate.
	anchorRow := -1
	var anchorBody string
	for i := 0; i < band; i++ {
		body := stripGutter(rows[top+i].runes, gutterWidth)
		t := strings.TrimRight(string(body), " ")
		if t != "" {
			anchorRow = i
			anchorBody = t
			break
		}
	}
	if anchorRow < 0 {
		return alignment{}
	}
	bestLine := 0
	bestRatio := 0.0
	for li, line := range lines {
		expanded := expandTabs(line, tabstop)
		ratio := prefixSimilarity(anchorBody, expanded)
		if ratio > bestRatio {
			bestRatio = ratio
			bestLine = li + 1
		}
	}
	if bestLine == 0 || bestRatio < 0.85 {
		return alignment{}
	}

	// Walk forward: each file line consumes ceil(width/bodyWidth) rows.
	rowToLine := make([]int, band)
	rowIdx := anchorRow
	fileIdx := bestLine
	for rowIdx < band && fileIdx-1 < len(lines) {
		ln := lines[fileIdx-1]
		expandedWidth := len([]rune(expandTabs(ln, tabstop)))
		segs := 1
		if expandedWidth > bodyWidth {
			segs = (expandedWidth + bodyWidth - 1) / bodyWidth
		}
		for s := 0; s < segs && rowIdx < band; s++ {
			// First segment is anchored to the file line; subsequent
			// segments are continuations (lineNo = 0 so detectWrap
			// recognises them).
			if s == 0 {
				rowToLine[rowIdx] = fileIdx
			} else {
				rowToLine[rowIdx] = 0
			}
			rowIdx++
		}
		fileIdx++
	}

	// Backfill rows before the anchor by walking backwards.
	bIdx := anchorRow - 1
	fIdx := bestLine - 1
	for bIdx >= 0 && fIdx >= 1 {
		rowToLine[bIdx] = fIdx
		bIdx--
		fIdx--
	}

	topFileLine := rowToLine[0]
	if topFileLine == 0 {
		// Find first non-zero entry.
		for _, v := range rowToLine {
			if v > 0 {
				topFileLine = v
				break
			}
		}
	}

	coverage := scoreWrapAlignment(rows, top, gutterWidth, rowToLine, lines, tabstop, bodyWidth)
	return alignment{
		ok:          coverage >= 0.6,
		rowToLine:   rowToLine,
		topFileLine: topFileLine,
		tabstop:     tabstop,
		coverage:    coverage,
	}
}

// scoreWrapAlignment is like scoreAlignment but treats consecutive rows
// that have lineNo == 0 as continuation segments of the previous line.
func scoreWrapAlignment(
	rows []extractedRow,
	top int,
	gutterWidth int,
	rowToLine []int,
	lines []string,
	tabstop int,
	bodyWidth int,
) float64 {
	considered, matched := 0, 0
	prevLine := 0
	segIdx := 0
	for i, ln := range rowToLine {
		row := rows[top+i]
		body := stripGutter(row.runes, gutterWidth)
		bodyStr := strings.TrimRight(string(body), " ")
		if ln > 0 {
			prevLine = ln
			segIdx = 0
		} else {
			segIdx++
		}
		line := prevLine
		if line < 1 || line > len(lines) {
			if bodyStr == "" {
				continue
			}
			considered++
			continue
		}
		if bodyStr == "" {
			continue
		}
		expanded := expandTabs(lines[line-1], tabstop)
		// Compare only the slice of expanded covered by this segment.
		start := segIdx * bodyWidth
		end := start + bodyWidth
		runes := []rune(expanded)
		if start > len(runes) {
			start = len(runes)
		}
		if end > len(runes) {
			end = len(runes)
		}
		expSegment := string(runes[start:end])
		considered++
		if prefixSimilarity(bodyStr, expSegment) >= 0.85 {
			matched++
		}
	}
	if considered == 0 {
		return 0
	}
	return float64(matched) / float64(considered)
}

// prefixSimilarity reports how much of the (shorter) text matches the
// prefix of the other. Returns 1.0 when one string is a prefix of the
// other, regardless of the longer string's full length.
func prefixSimilarity(a, b string) float64 {
	ar := []rune(strings.TrimRight(a, " "))
	br := []rune(strings.TrimRight(b, " "))
	if len(ar) == 0 && len(br) == 0 {
		return 1
	}
	if len(ar) == 0 || len(br) == 0 {
		return 0
	}
	n := len(ar)
	if len(br) < n {
		n = len(br)
	}
	common := 0
	for i := 0; i < n; i++ {
		if ar[i] != br[i] {
			break
		}
		common++
	}
	short := len(ar)
	if len(br) < short {
		short = len(br)
	}
	if short == 0 {
		return 0
	}
	return float64(common) / float64(short)
}

func alignByContentForTabstop(
	rows []extractedRow,
	top, bot int,
	gutterWidth int,
	lines []string,
	tabstop int,
) alignment {
	band := bot - top + 1
	if band <= 0 {
		return alignment{}
	}

	// Try every plausible topFileLine and keep the one with the best
	// global score. Anchoring on the first non-empty row alone is
	// unreliable on rendered bands that begin with ambiguous content
	// (e.g. a lone `}` matches every top-level closing brace in the
	// file), so we let the whole band vote on the offset instead.
	bestTop := 0
	bestCoverage := 0.0
	var bestRowToLine []int
	scratch := make([]int, band)
	for topFileLine := 1; topFileLine <= len(lines); topFileLine++ {
		for i := 0; i < band; i++ {
			ln := topFileLine + i
			if ln >= 1 && ln <= len(lines) {
				scratch[i] = ln
			} else {
				scratch[i] = 0
			}
		}
		coverage := scoreAlignment(rows, top, gutterWidth, scratch, lines, tabstop)
		if coverage > bestCoverage {
			bestCoverage = coverage
			bestTop = topFileLine
			bestRowToLine = append(bestRowToLine[:0], scratch...)
		}
	}
	if bestTop == 0 || bestCoverage < 0.6 {
		return alignment{}
	}
	return alignment{
		ok:          true,
		rowToLine:   bestRowToLine,
		topFileLine: bestTop,
		tabstop:     tabstop,
		coverage:    bestCoverage,
	}
}

// scoreAlignment returns the fraction of non-empty rows whose rendered
// body (after stripping gutterWidth runes) matches the corresponding
// file line under the given tabstop. Rows are taken from rowToLine
// starting at terminal row top.
func scoreAlignment(
	rows []extractedRow,
	top int,
	gutterWidth int,
	rowToLine []int,
	lines []string,
	tabstop int,
) float64 {
	considered, matched := 0, 0
	for i, line := range rowToLine {
		row := rows[top+i]
		body := stripGutter(row.runes, gutterWidth)
		bodyStr := strings.TrimRight(string(body), " ")
		if line < 1 || line > len(lines) {
			// Predicted past EOF: ignore these rows in the score.
			// Editors fill the visible region with placeholder glyphs
			// (e.g. "~" or "+", possibly a status line) that we do
			// not want to penalize the alignment for.
			continue
		}
		if bodyStr == "" {
			continue
		}
		expected := expandTabs(lines[line-1], tabstop)
		considered++
		if similarity(bodyStr, expected) >= 0.85 {
			matched++
		}
	}
	if considered == 0 {
		return 0
	}
	return float64(matched) / float64(considered)
}

// stripGutter returns the runes of row after dropping gutterWidth
// visual columns from the left.
func stripGutter(runes []rune, gutterWidth int) []rune {
	if gutterWidth <= 0 || gutterWidth >= len(runes) {
		if gutterWidth >= len(runes) {
			return nil
		}
		return runes
	}
	return runes[gutterWidth:]
}

// similarity returns a coarse ratio in [0, 1] for how well rendered
// matches expected. We use prefix matching plus length ratio rather
// than full Levenshtein: editors rarely rewrite the middle of a line,
// so prefix similarity is a cheap and effective signal.
func similarity(rendered, expected string) float64 {
	if rendered == "" && expected == "" {
		return 1
	}
	rendered = strings.TrimRight(rendered, " ")
	expected = strings.TrimRight(expected, " ")
	if rendered == expected {
		return 1
	}
	if rendered == "" || expected == "" {
		return 0
	}
	// Compare in runes for stable behavior with wide chars.
	rr := []rune(rendered)
	er := []rune(expected)
	common := 0
	n := len(rr)
	if len(er) < n {
		n = len(er)
	}
	for i := 0; i < n; i++ {
		if rr[i] != er[i] {
			break
		}
		common++
	}
	maxLen := len(rr)
	if len(er) > maxLen {
		maxLen = len(er)
	}
	if maxLen == 0 {
		return 1
	}
	return float64(common) / float64(maxLen)
}
