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

import "github.com/unstablebuild/rune-go-sdk/term"

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
	lines [][]term.Cell,
	tabstopHints []int,
	slab *Slab,
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
		score := scoreAlignment(rows, top, gut.width, rowToLine, lines, ts, slab)
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
	lines [][]term.Cell,
	tabstopHints []int,
	slab *Slab,
) alignment {
	band := bot - top + 1
	if band <= 0 {
		return alignment{}
	}

	bestTab := tabstopHints[0]
	var best alignment
	for _, ts := range tabstopHints {
		a := alignByContentForTabstop(rows, top, bot, gutterWidth, lines, ts, slab)
		// When the simple 1:1 alignment underperforms, try a
		// wrap-aware variant that lets a single file line span
		// multiple consecutive rows.
		if !a.ok || a.coverage < 0.95 {
			b := alignWithWrap(rows, top, bot, gutterWidth, lines, ts, slab)
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
// Rather than trust a single anchor row (a lone "}" or a blank line
// matches many candidates), it votes over every plausible top file
// line: for each candidate it simulates width-based soft wrapping down
// the band and keeps the candidate whose simulated row→line mapping
// best matches the rendered rows. This mirrors alignByContentForTabstop
// but accounts for lines that span multiple visual rows.
func alignWithWrap(
	rows []extractedRow,
	top, bot int,
	gutterWidth int,
	lines [][]term.Cell,
	tabstop int,
	slab *Slab,
) alignment {
	band := bot - top + 1
	if band <= 0 {
		return alignment{}
	}
	bodyWidth := bandBodyWidth(rows, top, bot, gutterWidth)
	if bodyWidth <= 0 {
		return alignment{}
	}

	// Precompute everything that is invariant across candidate top
	// lines so the O(lines) vote below does no per-candidate
	// allocation: the expanded rune form and segment count of each
	// file line, and the trimmed body text of each band row. The
	// expanded lines are shared with the 1:1 pass via the slab.
	expanded := slab.expandLinesFor(lines, tabstop)
	segCounts := slab.segCountBuf(len(lines))
	for i := range lines {
		segs := 1
		if len(expanded[i]) > bodyWidth {
			segs = (len(expanded[i]) + bodyWidth - 1) / bodyWidth
		}
		segCounts[i] = segs
	}
	bodies := slab.bodyBuf(band)
	for i := 0; i < band; i++ {
		bodies[i] = trimRightSpace(stripGutter(rows[top+i].runes, gutterWidth))
	}

	bestTop := 0
	bestCoverage := 0.0
	var bestRowToLine []int
	scratch := slab.rowToLineBuf(band)
	for topFileLine := 1; topFileLine <= len(lines); topFileLine++ {
		simulateWrapInto(scratch, topFileLine, segCounts)
		coverage := scoreWrapRows(bodies, scratch, expanded, bodyWidth)
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

// simulateWrapInto fills dst (length band) with the row→line mapping
// produced by walking file lines from topFileLine, consuming
// segCounts[line] visual rows per line. The first row of each line holds
// its 1-based file line; soft-wrap continuation rows hold 0 so detectWrap
// recognises them. Rows past EOF hold 0.
func simulateWrapInto(dst []int, topFileLine int, segCounts []int) {
	rowIdx := 0
	fileIdx := topFileLine
	for rowIdx < len(dst) && fileIdx-1 < len(segCounts) {
		segs := segCounts[fileIdx-1]
		for s := 0; s < segs && rowIdx < len(dst); s++ {
			if s == 0 {
				dst[rowIdx] = fileIdx
			} else {
				dst[rowIdx] = 0
			}
			rowIdx++
		}
		fileIdx++
	}
	for ; rowIdx < len(dst); rowIdx++ {
		dst[rowIdx] = 0
	}
}

// bandBodyWidth returns the visible body width (grid width minus gutter)
// of the content band, taken from the widest row so a short first row
// does not under-estimate the terminal width.
func bandBodyWidth(rows []extractedRow, top, bot, gutterWidth int) int {
	w := 0
	for y := top; y <= bot && y < len(rows); y++ {
		if n := len(rows[y].runeColMap); n > w {
			w = n
		}
	}
	return w - gutterWidth
}

// scoreWrapRows scores a wrap-aware rowToLine mapping against
// precomputed inputs without allocating: bodies[i] is the trimmed
// rendered runes of band row i, expanded[l-1] is the tab-expanded rune
// form of file line l, and bodyWidth is the visible body width that
// bounds each wrap segment. Rows with rowToLine == 0 are treated as
// continuation segments of the previous line. It is the hot inner loop
// of alignWithWrap's candidate vote.
func scoreWrapRows(bodies [][]rune, rowToLine []int, expanded [][]rune, bodyWidth int) float64 {
	considered, matched := 0, 0
	prevLine := 0
	segIdx := 0
	for i, ln := range rowToLine {
		body := bodies[i]
		if ln > 0 {
			prevLine = ln
			segIdx = 0
		} else {
			segIdx++
		}
		line := prevLine
		if line < 1 || line > len(expanded) {
			// Below-EOF filler is neutral; real text past EOF is a
			// mismatch (see isFillerBody / scoreAlignment).
			if !isFillerBodyRunes(body) {
				considered++
			}
			continue
		}
		if len(body) == 0 {
			continue
		}
		runes := expanded[line-1]
		start := segIdx * bodyWidth
		end := start + bodyWidth
		if start > len(runes) {
			start = len(runes)
		}
		if end > len(runes) {
			end = len(runes)
		}
		considered++
		if prefixSimilarityRunes(body, runes[start:end]) >= 0.85 {
			matched++
		}
	}
	if considered == 0 {
		return 0
	}
	return float64(matched) / float64(considered)
}

// prefixSimilarityRunes reports the fraction of the shorter slice's
// leading runes that match the other's prefix. a is already trimmed;
// trailing spaces of b (a width-padded wrap segment) are ignored.
// Returns 1.0 when one slice is a prefix of the other. Allocation-free.
func prefixSimilarityRunes(a, b []rune) float64 {
	bn := len(b)
	for bn > 0 && b[bn-1] == ' ' {
		bn--
	}
	if len(a) == 0 && bn == 0 {
		return 1
	}
	if len(a) == 0 || bn == 0 {
		return 0
	}
	short := len(a)
	if bn < short {
		short = bn
	}
	common := 0
	for i := 0; i < short; i++ {
		if a[i] != b[i] {
			break
		}
		common++
	}
	return float64(common) / float64(short)
}

func alignByContentForTabstop(
	rows []extractedRow,
	top, bot int,
	gutterWidth int,
	lines [][]term.Cell,
	tabstop int,
	slab *Slab,
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
	//
	// The expanded file lines (shared with the wrap pass via the slab)
	// and trimmed row bodies are precomputed once so this O(lines*band)
	// vote allocates nothing per candidate. The scorer ignores trailing
	// spaces, so the expanded lines are used untrimmed straight from the
	// slab arena.
	expanded := slab.expandLinesFor(lines, tabstop)
	bodies := slab.bodyBuf(band)
	for i := 0; i < band; i++ {
		bodies[i] = trimRightSpace(stripGutter(rows[top+i].runes, gutterWidth))
	}

	bestTop := 0
	bestCoverage := 0.0
	for topFileLine := 1; topFileLine <= len(lines); topFileLine++ {
		coverage := scoreRows1to1(bodies, expanded, topFileLine)
		if coverage > bestCoverage {
			bestCoverage = coverage
			bestTop = topFileLine
		}
	}
	if bestTop == 0 || bestCoverage < 0.6 {
		return alignment{}
	}
	rowToLine := make([]int, band)
	for i := 0; i < band; i++ {
		ln := bestTop + i
		if ln >= 1 && ln <= len(lines) {
			rowToLine[i] = ln
		}
	}
	return alignment{
		ok:          true,
		rowToLine:   rowToLine,
		topFileLine: bestTop,
		tabstop:     tabstop,
		coverage:    bestCoverage,
	}
}

// scoreRows1to1 scores a non-wrap 1:1 mapping (band row i -> file line
// topFileLine+i) against precomputed trimmed bodies and trimmed
// expanded file lines, allocating nothing. It mirrors scoreAlignment's
// rules: empty body rows are skipped, real content predicted past EOF
// counts as a mismatch while below-EOF filler is neutral, and a row
// matches when its prefix-ratio against the expected line is >= 0.85.
func scoreRows1to1(bodies, expanded [][]rune, topFileLine int) float64 {
	considered, matched := 0, 0
	for i, body := range bodies {
		line := topFileLine + i
		if line < 1 || line > len(expanded) {
			if !isFillerBodyRunes(body) {
				considered++
			}
			continue
		}
		if len(body) == 0 {
			continue
		}
		considered++
		if runePrefixRatioMaxLen(body, expanded[line-1]) >= 0.85 {
			matched++
		}
	}
	if considered == 0 {
		return 0
	}
	return float64(matched) / float64(considered)
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
	lines [][]term.Cell,
	tabstop int,
	slab *Slab,
) float64 {
	considered, matched := 0, 0
	expanded := slab.expandLinesFor(lines, tabstop)
	for i, line := range rowToLine {
		row := rows[top+i]
		body := trimRightSpace(stripGutter(row.runes, gutterWidth))
		if line < 1 || line > len(lines) {
			// Predicted past EOF. Genuine below-EOF filler (blank rows,
			// or single-glyph markers like vim's "~") is ignored. But a
			// row that still carries real text means this candidate
			// pushed visible content past the end of the file — strong
			// evidence the anchor is wrong — so count it as a mismatch.
			// Without this, anchoring on the last file line scores a
			// vacuous 1.0 from its single in-bounds row.
			if !isFillerBodyRunes(body) {
				considered++
			}
			continue
		}
		if len(body) == 0 {
			continue
		}
		considered++
		if runePrefixRatioMaxLen(body, expanded[line-1]) >= 0.85 {
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

// isFillerBodyRunes reports whether a trimmed row body is editor filler
// rendered below the end of the file rather than real file content.
func isFillerBodyRunes(body []rune) bool {
	if len(body) == 0 {
		return true
	}
	if len(body) > 2 {
		return false
	}
	for _, c := range body {
		switch c {
		case '~', '+', '-', '|':
		default:
			return false
		}
	}
	return true
}

// trimRightSpace returns r with trailing spaces dropped. It reslices in
// place and never allocates.
func trimRightSpace(r []rune) []rune {
	n := len(r)
	for n > 0 && r[n-1] == ' ' {
		n--
	}
	return r[:n]
}

// runePrefixRatioMaxLen mirrors similarity for two rune slices: the
// count of common leading runes divided by the longer length, ignoring
// trailing spaces on both sides. Allocation-free, so callers may pass
// width-padded slices straight from the expansion arena.
func runePrefixRatioMaxLen(a, b []rune) float64 {
	an := trimmedLen(a)
	bn := trimmedLen(b)
	if an == 0 && bn == 0 {
		return 1
	}
	if an == 0 || bn == 0 {
		return 0
	}
	n := an
	if bn < n {
		n = bn
	}
	common := 0
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			break
		}
		common++
	}
	maxLen := an
	if bn > maxLen {
		maxLen = bn
	}
	return float64(common) / float64(maxLen)
}

// trimmedLen returns the length of r with trailing spaces excluded.
func trimmedLen(r []rune) int {
	n := len(r)
	for n > 0 && r[n-1] == ' ' {
		n--
	}
	return n
}
