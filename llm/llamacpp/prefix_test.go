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


package llamacpp

import "testing"

func TestCommonPrefixLen(t *testing.T) {
	tests := []struct {
		name string
		a, b []int32
		want int
	}{
		{"both nil", nil, nil, 0},
		{"left empty", nil, []int32{1, 2, 3}, 0},
		{"right empty", []int32{1, 2, 3}, nil, 0},
		{"equal", []int32{1, 2, 3}, []int32{1, 2, 3}, 3},
		{"left shorter", []int32{1, 2}, []int32{1, 2, 3}, 2},
		{"right shorter", []int32{1, 2, 3}, []int32{1, 2}, 2},
		{"diverge mid", []int32{1, 2, 9, 4}, []int32{1, 2, 3, 4}, 2},
		{"diverge at 0", []int32{9}, []int32{1}, 0},
		{"one common", []int32{5, 0, 0}, []int32{5, 1, 2}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := commonPrefixLen(tc.a, tc.b); got != tc.want {
				t.Fatalf("commonPrefixLen(%v, %v) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}


// TestPlanCacheReuse pins down the shift planner used by
// applyCacheReuse. Mirrors the canonical scenario from
// server-context.cpp: the prompt has grown a chunk in the middle and
// we want to recover the tail without re-evaluating it.
func TestPlanCacheReuse(t *testing.T) {
	tests := []struct {
		name      string
		cached    []int32
		prompt    []int32
		nPast     int
		minMatch  int
		wantShift []cacheReuseShift
		wantNPast int
	}{
		{
			name:      "minMatch=0 disables shift-reuse",
			cached:    []int32{1, 2, 3, 4},
			prompt:    []int32{1, 9, 3, 4},
			nPast:     1,
			minMatch:  0,
			wantShift: nil,
			wantNPast: 1,
		},
		{
			name: "tail chunk recovered after divergence",
			// Cache had two stale tokens (1010, 1011) that the new
			// prompt drops; the tail past them is identical, so we
			// expect a shift that re-bases the tail four tokens
			// earlier in the cache.
			cached:    []int32{1, 2, 1010, 1011, 5, 6, 7, 8},
			prompt:    []int32{1, 2, 5, 6, 7, 8},
			nPast:     2,
			minMatch:  3,
			wantShift: []cacheReuseShift{{srcStart: 4, dstStart: 2, count: 4}},
			wantNPast: 6,
		},
		{
			name:      "no chunk meets the minMatch threshold",
			cached:    []int32{1, 2, 3, 4},
			prompt:    []int32{1, 9, 3, 4},
			nPast:     1,
			minMatch:  3,
			wantShift: nil,
			wantNPast: 1,
		},
		{
			name:      "single stale token in cache: tail recovered",
			cached:    []int32{1, 2, 99, 3, 4},
			prompt:    []int32{1, 2, 3, 4},
			nPast:     2,
			minMatch:  2,
			wantShift: []cacheReuseShift{{srcStart: 3, dstStart: 2, count: 2}},
			wantNPast: 4,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cached := append([]int32(nil), tc.cached...)
			gotShifts, gotNPast := planCacheReuse(cached, tc.prompt, tc.nPast, tc.minMatch)
			if gotNPast != tc.wantNPast {
				t.Fatalf("nPast = %d, want %d", gotNPast, tc.wantNPast)
			}
			if len(gotShifts) != len(tc.wantShift) {
				t.Fatalf("shifts = %+v, want %+v", gotShifts, tc.wantShift)
			}
			for i := range gotShifts {
				if gotShifts[i] != tc.wantShift[i] {
					t.Fatalf("shift[%d] = %+v, want %+v", i, gotShifts[i], tc.wantShift[i])
				}
			}
		})
	}
}

func TestResolvePrefixReuse(t *testing.T) {
	// resolvePrefixReuse encapsulates the decision that CreateCompletion
	// makes each turn: given the cached tokens and the new prompt, how many
	// tokens can be reused, and how many must be (re-)evaluated.
	//
	// Invariants:
	//   - numPast ≤ len(prompt)-1  (always leave ≥1 token to sample from)
	//   - numPast ≤ len(cached)    (cannot reuse more than what's cached)
	//   - tokens[:numPast] == cached[:numPast]
	tests := []struct {
		name        string
		cached      []int32
		prompt      []int32
		wantNumPast int
	}{
		{
			name:        "first turn, no cache",
			cached:      nil,
			prompt:      []int32{1, 2, 3, 4, 5},
			wantNumPast: 0,
		},
		{
			name:        "full prefix match — back off by one",
			cached:      []int32{1, 2, 3, 4, 5},
			prompt:      []int32{1, 2, 3, 4, 5, 6, 7, 8},
			wantNumPast: 5,
		},
		{
			name:        "identical prompt — back off by one to reserve sample",
			cached:      []int32{1, 2, 3, 4, 5},
			prompt:      []int32{1, 2, 3, 4, 5},
			wantNumPast: 4,
		},
		{
			name:        "partial match",
			cached:      []int32{1, 2, 3, 9, 9},
			prompt:      []int32{1, 2, 3, 4, 5},
			wantNumPast: 3,
		},
		{
			name:        "complete divergence",
			cached:      []int32{9, 9, 9},
			prompt:      []int32{1, 2, 3},
			wantNumPast: 0,
		},
		{
			name:        "prompt shorter than cache but matches",
			cached:      []int32{1, 2, 3, 4, 5},
			prompt:      []int32{1, 2, 3},
			wantNumPast: 2,
		},
		{
			name:        "single-token prompt",
			cached:      []int32{1},
			prompt:      []int32{1},
			wantNumPast: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolvePrefixReuse(tc.cached, tc.prompt)
			if got != tc.wantNumPast {
				t.Fatalf("resolvePrefixReuse() = %d, want %d", got, tc.wantNumPast)
			}
			if got > len(tc.prompt)-1 {
				t.Fatalf("invariant broken: numPast %d > len(prompt)-1 %d", got, len(tc.prompt)-1)
			}
			if got > len(tc.cached) {
				t.Fatalf("invariant broken: numPast %d > len(cached) %d", got, len(tc.cached))
			}
			for i := 0; i < got; i++ {
				if tc.prompt[i] != tc.cached[i] {
					t.Fatalf("invariant broken: mismatch at %d", i)
				}
			}
		})
	}
}
