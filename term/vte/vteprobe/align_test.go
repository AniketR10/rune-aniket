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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAlignByGutter(t *testing.T) {
	t.Parallel()

	lines := cellLines([]string{
		"package main",
		"",
		"import \"fmt\"",
		"",
		"func main() {",
		"\tfmt.Println(\"hello\")",
		"}",
	})
	rows := []extractedRow{
		makeRow(" 1 package main"),
		makeRow(" 2 "),
		makeRow(" 3 import \"fmt\""),
		makeRow(" 4 "),
		makeRow(" 5 func main() {"),
		makeRow(" 6     fmt.Println(\"hello\")"),
		makeRow(" 7 }"),
	}
	g := detectGutter(rows, 0, len(rows)-1)
	assert.True(t, g.present)

	a := alignByGutter(rows, 0, len(rows)-1, g, lines, []int{4, 2, 8}, NewSlab())
	assert.True(t, a.ok, "expected aligned, coverage=%v", a.coverage)
	assert.Equal(t, 1, a.topFileLine)
	assert.Equal(t, 4, a.tabstop, "should infer tabstop 4 because line 6 renders 4 spaces before fmt")
}

func TestAlignByContent(t *testing.T) {
	t.Parallel()

	lines := cellLines([]string{
		"package main",
		"",
		"import \"fmt\"",
		"",
		"func main() {",
		"\tfmt.Println(\"hello\")",
		"}",
	})
	rows := []extractedRow{
		makeRow("import \"fmt\""),
		makeRow(""),
		makeRow("func main() {"),
		makeRow("    fmt.Println(\"hello\")"),
		makeRow("}"),
	}

	a := alignByContentWithGutter(rows, 0, len(rows)-1, 0, lines, []int{4, 2, 8}, NewSlab())
	assert.True(t, a.ok, "expected aligned, coverage=%v", a.coverage)
	assert.Equal(t, 3, a.topFileLine)
	assert.Equal(t, 4, a.tabstop)
}

// TestAlignWithWrapAmbiguousAnchor reproduces the soft-wrap regression
// where the visible band starts on an ambiguous row (a lone "}") that
// prefix-matches many file lines. Anchoring on that single row picked
// the wrong file line; voting over every candidate start line recovers
// the correct anchor even though long lines wrap across multiple rows.
func TestAlignWithWrapAmbiguousAnchor(t *testing.T) {
	t.Parallel()

	// A file with several lone "}" lines so the anchor row is ambiguous,
	// and one long line that wraps at the 20-column body width.
	long := "\tx := aaaaaaaaaa + bbbbbbbbbb + cccccccccc" // > 20 cols expanded
	lines := cellLines([]string{
		"func a() {", // 1
		"\treturn",   // 2
		"}",          // 3
		"func b() {", // 4
		long,         // 5 (wraps)
		"}",          // 6
		"func c() {", // 7
		"\treturn",   // 8
		"}",          // 9
	})
	// Render the band starting at file line 6 (a lone "}"), which also
	// appears at lines 3 and 9. Width 20 forces the long line to wrap,
	// but here the band starts after it.
	const width = 20
	rows := []extractedRow{
		makeRowW("}", width),              // line 6
		makeRowW("func c() {", width),     // line 7
		makeRowW("        return", width), // line 8 (tab -> 8 spaces)
		makeRowW("}", width),              // line 9
	}

	a := alignByContentWithGutter(rows, 0, len(rows)-1, 0, lines, []int{8, 4, 2}, NewSlab())
	assert.True(t, a.ok, "expected aligned, coverage=%v", a.coverage)
	assert.Equal(t, 6, a.topFileLine,
		"should anchor on line 6, not the ambiguous lines 3 or 9")
}

// TestAlignTailViewNotAnchoredPastEOF reproduces the regression where a
// view scrolled near end-of-file anchored on the very last line. With a
// lone "}" at the top of the band, the 1:1 scorer mapped the rest of the
// band past EOF and skipped those rows, so anchoring on the final line
// scored a vacuous 1.0 from its single in-bounds row and beat the
// correct wrap alignment. Real content predicted past EOF must count as
// a mismatch (only blank/"~" filler is neutral).
func TestAlignTailViewNotAnchoredPastEOF(t *testing.T) {
	t.Parallel()

	// One long line (line 2) wraps across two rows at the 20-col body
	// width. The band starts on a lone "}" (line 3) that recurs at the
	// end of the file (line 6, the last line).
	const width = 20
	long := "\tprint(aaaaaaaaaa, bbbbbbbbbb)" // expands well past 20 cols
	lines := cellLines([]string{
		"func a() {", // 1
		long,         // 2 (wraps -> 2 rows)
		"}",          // 3
		"func b() {", // 4
		"\tnoop()",   // 5
		"}",          // 6 (last line, recurs)
	})
	// Visible band: line 3 "}" at the top, then 4,5,6 and "~" filler.
	// A correct 1:1 anchor at line 3 also matches; but to force the
	// wrap path to be the discriminator we include the wrapped line's
	// continuation as if the band scrolled to show line 3 first — the
	// regression is that anchoring on the last line (6) scored a
	// vacuous 1.0 by mapping rows 1..n past EOF and skipping them.
	rows := []extractedRow{
		makeRowW("}", width),              // line 3
		makeRowW("func b() {", width),     // line 4
		makeRowW("        noop()", width), // line 5 (tab -> 8 spaces)
		makeRowW("}", width),              // line 6
		makeRowW("~", width),              // filler past EOF
		makeRowW("~", width),              // filler past EOF
		makeRowW("~", width),              // filler past EOF
	}

	// Score the vacuous tail anchor (top line = 6) directly: only its
	// single "}" row is in bounds, the rest are real "~" filler. With
	// the fix this must NOT reach a perfect score that out-ranks the
	// true anchor at line 3.
	band := len(rows)
	tailMap := make([]int, band)
	tailMap[0] = 6 // "}" on the last line; rows 1..n fall past EOF
	tailCov := scoreAlignment(rows, 0, 0, tailMap, lines, 8, NewSlab())

	trueMap := []int{3, 4, 5, 6, 0, 0, 0}
	trueCov := scoreAlignment(rows, 0, 0, trueMap, lines, 8, NewSlab())

	assert.Greater(t, trueCov, tailCov,
		"true anchor (line 3) must score above the vacuous tail anchor (line 6): true=%v tail=%v",
		trueCov, tailCov)

	a := alignByContentWithGutter(rows, 0, len(rows)-1, 0, lines, []int{8, 4, 2}, NewSlab())
	assert.True(t, a.ok, "expected aligned, coverage=%v", a.coverage)
	assert.Equal(t, 3, a.topFileLine,
		"should anchor on line 3, not the last line 6")
}
