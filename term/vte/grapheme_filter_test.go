// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package vte

import (
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/rivo/uniseg"
	"github.com/unstablebuild/rune-go-sdk/term/graphemecluster"
)

// TestMayExtendClusterNeverRejectsAContinuation sweeps every assignable
// codepoint against a set of bases chosen to activate each stateful
// UAX #29 rule. mayExtendCluster gates the full segmentation probe in
// mergeContinuation, so a false negative would silently split a
// grapheme cluster across two cells; a false positive only costs the
// probe it was meant to avoid.
func TestMayExtendClusterNeverRejectsAContinuation(t *testing.T) {
	t.Parallel()

	bases := []struct {
		name string
		text string
	}{
		{"latin", "a"},
		{"cjk", "中"},
		{"pictographic", "🙂"},
		{"zwj_sequence", "🙂\u200d"},
		{"regional_indicator", "🇦"},
		{"hangul_lead", "ᄀ"},
		{"hangul_syllable", "가"},
		{"devanagari", "क"},
		{"carriage_return", "\r"},
	}

	for _, base := range bases {
		t.Run(base.name, func(t *testing.T) {
			t.Parallel()
			last, _ := utf8.DecodeLastRuneInString(base.text)
			for c := rune(0x300); c <= 0x10FFFF; c++ {
				if c >= 0xD800 && c <= 0xDFFF {
					continue
				}
				_, rest, _, _ := uniseg.FirstGraphemeClusterInString(
					base.text+string(c), -1)
				if rest != "" {
					continue
				}
				width := graphemecluster.StringWidth(string(c))
				if !mayContinueAnyCluster(c, width) {
					t.Fatalf("mayContinueAnyCluster(%U, width=%d) = false, but "+
						"uniseg merges it into the preceding cluster %q; the "+
						"cluster would be split across cells", c, width, base.text)
				}
				if !mayExtendCluster(last, c, width) {
					t.Fatalf("mayExtendCluster(%U, %U, width=%d) = false, but "+
						"uniseg merges it into the preceding cluster %q; the "+
						"cluster would be split across cells",
						last, c, width, base.text)
				}
			}
		})
	}
}

// TestMergeContinuationClusters pins the end-to-end behaviour the filter
// protects: a cluster typed one codepoint at a time must land in a
// single cell.
func TestMergeContinuationClusters(t *testing.T) {
	t.Parallel()

	suite := []struct {
		text  string
		cells int
	}{
		{"e\u0301", 1},            // combining acute
		{"❤\ufe0f", 1},            // variation selector 16
		{"👍\U0001F3FD", 1},        // skin-tone modifier
		{"👨\u200d👩\u200d👧", 1},    // ZWJ family
		{"🇺🇸", 1},                 // regional indicator pair
		{"각", 1},                  // precomposed hangul syllable
		{"\u1100\u1161\u11A8", 1}, // decomposed hangul jamo
		{"中文", 2},                 // unrelated CJK: never merges
		{"🙂🙂", 2},                 // unrelated pictographs
	}

	for i, test := range suite {
		t.Run(fmt.Sprintf("%d_%q", i, test.text), func(t *testing.T) {
			t.Parallel()
			ph := newBenchParserHandler(40, 4)
			for _, c := range test.text {
				ph.Input(c)
			}
			row := ph.sync.primBuf.Cells.RawCells()[0]
			var got int
			for _, cell := range row {
				if cell.Ch != ' ' && cell.Ch != 0 {
					got++
				}
			}
			if got != test.cells {
				t.Fatalf("%q occupied %d cells, want %d", test.text, got, test.cells)
			}
		})
	}
}
