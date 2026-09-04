// Copyright (C) 2017-2026 Unstable Build, LLC
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

package applypatch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSeekSequence(t *testing.T) {
	tests := []struct {
		name      string
		fileLines []string
		pattern   []string
		start     int
		want      int
	}{
		{
			name:      "exact match at start",
			fileLines: []string{"alpha", "beta", "gamma"},
			pattern:   []string{"alpha", "beta"},
			want:      0,
		},
		{
			name:      "exact match at offset",
			fileLines: []string{"alpha", "beta", "gamma", "delta"},
			pattern:   []string{"gamma", "delta"},
			want:      2,
		},
		{
			name:      "trailing whitespace tolerance",
			fileLines: []string{"alpha  ", "beta\t", "gamma"},
			pattern:   []string{"alpha", "beta"},
			want:      0,
		},
		{
			name:      "leading and trailing whitespace tolerance",
			fileLines: []string{"  alpha  ", "\tbeta\t", "gamma"},
			pattern:   []string{"alpha", "beta"},
			want:      0,
		},
		{
			name:      "no match returns -1",
			fileLines: []string{"alpha", "beta", "gamma"},
			pattern:   []string{"delta", "epsilon"},
			want:      -1,
		},
		{
			name:      "empty pattern returns start",
			fileLines: []string{"alpha", "beta"},
			pattern:   []string{},
			want:      0,
		},
		{
			name:      "empty pattern with start offset",
			fileLines: []string{"alpha", "beta"},
			pattern:   []string{},
			start:     1,
			want:      1,
		},
		{
			name:      "pattern longer than file",
			fileLines: []string{"alpha"},
			pattern:   []string{"alpha", "beta"},
			want:      -1,
		},
		{
			name:      "start offset skips earlier match",
			fileLines: []string{"a", "b", "a", "b"},
			pattern:   []string{"a", "b"},
			start:     1,
			want:      2,
		},
		{
			name:      "single line exact",
			fileLines: []string{"one", "two", "three"},
			pattern:   []string{"two"},
			want:      1,
		},
		{
			name:      "empty file with empty pattern",
			fileLines: []string{},
			pattern:   []string{},
			want:      0,
		},
		{
			name:      "empty file nonempty pattern",
			fileLines: []string{},
			pattern:   []string{"alpha"},
			want:      -1,
		},
		{
			name:      "match at end of file",
			fileLines: []string{"a", "b", "c"},
			pattern:   []string{"c"},
			want:      2,
		},
		{
			name:      "exact preferred over fuzzy when both present",
			fileLines: []string{"alpha ", "alpha", "beta"},
			pattern:   []string{"alpha", "beta"},
			want:      1,
		},
		{
			name:      "start beyond file length",
			fileLines: []string{"a", "b"},
			pattern:   []string{"a"},
			start:     5,
			want:      -1,
		},
		{
			name:      "blank line pattern matches blank file line",
			fileLines: []string{"a", "", "b"},
			pattern:   []string{""},
			want:      1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := seekSequence(tt.fileLines, tt.pattern, tt.start)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestLinesEqual(t *testing.T) {
	tests := []struct {
		name  string
		a, b  string
		level matchLevel
		want  bool
	}{
		{"exact match", "hello", "hello", matchExact, true},
		{"exact mismatch trailing", "hello ", "hello", matchExact, false},
		{"trimEnd match trailing", "hello  ", "hello", matchTrimEnd, true},
		{"trimEnd mismatch leading", "  hello", "hello", matchTrimEnd, false},
		{"trimAll match both", "  hello  ", "hello", matchTrimAll, true},
		{"trimAll mismatch content", "  hello  ", "world", matchTrimAll, false},
		{"exact empty strings", "", "", matchExact, true},
		{"trimEnd keeps leading whitespace significant", "\thello", "hello", matchTrimEnd, false},
		{"trimEnd carriage return stripped", "hello\r", "hello", matchTrimEnd, true},
		{"trimAll whitespace only equals empty", "  \t ", "", matchTrimAll, true},
		{"trimAll internal whitespace significant", "a b", "ab", matchTrimAll, false},
		{"unknown level returns false", "hello", "hello", matchLevel(99), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, linesEqual(tt.a, tt.b, tt.level))
		})
	}
}

func TestBestPartialMatch(t *testing.T) {
	tests := []struct {
		name      string
		fileLines []string
		pattern   []string
		start     int
		want      BestMatch
	}{
		{
			name:      "no lines match at all",
			fileLines: []string{"alpha", "beta", "gamma"},
			pattern:   []string{"delta", "epsilon"},
			want: BestMatch{
				Pos: 0, Matched: 0, Total: 2,
				ExpectedLine: "delta",
				ActualLine:   "alpha",
			},
		},
		{
			name:      "partial match first line only",
			fileLines: []string{"alpha", "beta", "gamma"},
			pattern:   []string{"alpha", "WRONG"},
			want: BestMatch{
				Pos: 0, Matched: 1, Total: 2,
				ExpectedLine: "WRONG",
				ActualLine:   "beta",
			},
		},
		{
			name:      "partial match two of three",
			fileLines: []string{"alpha", "beta", "gamma", "delta"},
			pattern:   []string{"beta", "gamma", "WRONG"},
			want: BestMatch{
				Pos: 1, Matched: 2, Total: 3,
				ExpectedLine: "WRONG",
				ActualLine:   "delta",
			},
		},
		{
			name:      "partial match extends past eof",
			fileLines: []string{"alpha", "beta"},
			pattern:   []string{"alpha", "beta", "gamma"},
			want: BestMatch{
				Pos: 0, Matched: 2, Total: 3,
				ExpectedLine: "gamma",
				PastEOF:      true,
			},
		},
		{
			name:      "empty pattern",
			fileLines: []string{"alpha"},
			pattern:   []string{},
			want:      BestMatch{Pos: 0, Matched: 0, Total: 0},
		},
		{
			name:      "fuzzy whitespace helps partial match",
			fileLines: []string{"  alpha  ", "beta", "gamma"},
			pattern:   []string{"alpha", "WRONG"},
			want: BestMatch{
				Pos: 0, Matched: 1, Total: 2,
				ExpectedLine: "WRONG",
				ActualLine:   "beta",
			},
		},
		{
			name:      "start offset respected",
			fileLines: []string{"alpha", "beta", "alpha", "WRONG"},
			pattern:   []string{"alpha", "beta"},
			start:     2,
			want: BestMatch{
				Pos: 2, Matched: 1, Total: 2,
				ExpectedLine: "beta",
				ActualLine:   "WRONG",
			},
		},
		{
			name:      "single line pattern no match",
			fileLines: []string{"alpha", "beta"},
			pattern:   []string{"gamma"},
			want: BestMatch{
				Pos: 0, Matched: 0, Total: 1,
				ExpectedLine: "gamma",
				ActualLine:   "alpha",
			},
		},
		{
			name:      "empty file nonempty pattern past eof",
			fileLines: []string{},
			pattern:   []string{"alpha"},
			want: BestMatch{
				Pos: 0, Matched: 0, Total: 1,
				ExpectedLine: "alpha",
				PastEOF:      true,
			},
		},
		{
			name:      "later full-length partial preferred",
			fileLines: []string{"alpha", "x", "alpha", "beta"},
			pattern:   []string{"alpha", "beta"},
			want: BestMatch{
				Pos: 2, Matched: 2, Total: 2,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bestPartialMatch(tt.fileLines, tt.pattern, tt.start)
			assert.Equal(t, tt.want, got)
		})
	}
}
