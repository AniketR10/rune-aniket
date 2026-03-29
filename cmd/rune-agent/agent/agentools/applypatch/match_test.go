// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bestPartialMatch(tt.fileLines, tt.pattern, tt.start)
			assert.Equal(t, tt.want, got)
		})
	}
}
