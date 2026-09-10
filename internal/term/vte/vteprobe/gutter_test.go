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

// makeRowW is makeRow but pads the row to width visual columns with
// spaces, so bandBodyWidth sees the intended terminal width rather than
// the trimmed content length.
func makeRowW(s string, width int) extractedRow {
	runes := []rune(s)
	for len(runes) < width {
		runes = append(runes, ' ')
	}
	return makeRow(string(runes))
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
