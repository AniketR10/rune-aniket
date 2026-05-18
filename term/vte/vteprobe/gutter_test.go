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
	"github.com/unstablebuild/rune-go-sdk/term"
)

func makeRow(s string) extractedRow {
	runes := []rune(s)
	r := extractedRow{
		runes:      runes,
		attrs:      make([]term.Attributes, len(runes)),
		runeColMap: make([]int, len(runes)),
	}
	for i := range runes {
		r.runeColMap[i] = i
	}
	return r
}

func TestParseLeadingLineNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input  string
		number int
		width  int
	}{
		{"  1 foo", 1, 4},
		{" 12 foo", 12, 4},
		{"345 foo", 345, 4},
		{"  1 | foo", 1, 6},
		{"  1 │ foo", 1, 6},
		{"no numbers", 0, 0},
		{"   ", 0, 0},
		{"42", 0, 0}, // no separator
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			n, w := parseLeadingLineNumber([]rune(tt.input))
			assert.Equal(t, tt.number, n, "number")
			assert.Equal(t, tt.width, w, "width")
		})
	}
}

func TestDetectGutter(t *testing.T) {
	t.Parallel()

	t.Run("absolute line numbers", func(t *testing.T) {
		t.Parallel()
		rows := []extractedRow{
			makeRow(" 1 func main() {"),
			makeRow(" 2     fmt.Println()"),
			makeRow(" 3 }"),
		}
		g := detectGutter(rows, 0, 2)
		assert.True(t, g.present)
		assert.Equal(t, 3, g.width) // " 1 " is 3 visual cols
		assert.Equal(t, []int{1, 2, 3}, g.lineNo)
	})

	t.Run("relative line numbers with current=0", func(t *testing.T) {
		t.Parallel()
		rows := []extractedRow{
			makeRow("2 foo"),
			makeRow("1 bar"),
			makeRow("0 baz"),
			makeRow("1 qux"),
		}
		// Relative line numbers include 0 for the current line; the
		// non-monotonic sequence is still a gutter because at least
		// one row has a positive number and every row contributes the
		// same prefix width.
		g := detectGutter(rows, 0, 3)
		assert.True(t, g.present)
		assert.Equal(t, 2, g.width)
	})

	t.Run("no gutter when zero increase", func(t *testing.T) {
		t.Parallel()
		rows := []extractedRow{
			makeRow("0 foo"),
			makeRow("0 bar"),
		}
		g := detectGutter(rows, 0, 1)
		assert.False(t, g.present)
	})

	t.Run("no gutter when content has no numbers", func(t *testing.T) {
		t.Parallel()
		rows := []extractedRow{
			makeRow("hello"),
			makeRow("world"),
		}
		g := detectGutter(rows, 0, 1)
		assert.False(t, g.present)
	})
}
