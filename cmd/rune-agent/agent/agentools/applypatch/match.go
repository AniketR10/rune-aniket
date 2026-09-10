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

package applypatch

import "strings"

type matchLevel int

const (
	matchExact   matchLevel = iota // byte-for-byte
	matchTrimEnd                   // trim trailing whitespace
	matchTrimAll                   // trim leading + trailing whitespace
)

func linesEqual(a, b string, level matchLevel) bool {
	switch level {
	case matchExact:
		return a == b
	case matchTrimEnd:
		return strings.TrimRight(a, " \t\r") == strings.TrimRight(b, " \t\r")
	case matchTrimAll:
		return strings.TrimSpace(a) == strings.TrimSpace(b)
	}
	return false
}

// seekSequence finds the first index in fileLines (starting at start)
// where all pattern lines match consecutively. It tries exact matching
// first, then trailing-whitespace-tolerant, then full-whitespace-tolerant.
// Returns -1 if no match is found.
func seekSequence(fileLines, pattern []string, start int) int {
	if len(pattern) == 0 {
		return start
	}

	for _, level := range []matchLevel{matchExact, matchTrimEnd, matchTrimAll} {
		if idx := seekAt(fileLines, pattern, start, level); idx >= 0 {
			return idx
		}
	}
	return -1
}

// BestMatch describes the best partial match found when a full match fails.
type BestMatch struct {
	// Pos is the file line index (0-based) where the best partial match starts.
	Pos int
	// Matched is how many consecutive pattern lines matched at Pos.
	Matched int
	// Total is the total number of pattern lines.
	Total int
	// ExpectedLine is the pattern line that first failed to match.
	ExpectedLine string
	// ActualLine is the corresponding file line at the point of divergence.
	// Empty if the divergence is past end-of-file.
	ActualLine string
	// PastEOF is true when the pattern extends beyond the end of the file.
	PastEOF bool
}

// bestPartialMatch scans fileLines from start for the position where the
// most consecutive pattern lines match. It uses the same three-tier
// matching cascade as seekSequence. The result helps produce actionable
// error messages when a full match is not found.
func bestPartialMatch(fileLines, pattern []string, start int) BestMatch {
	if len(pattern) == 0 {
		return BestMatch{Pos: start, Matched: 0, Total: 0}
	}

	best := BestMatch{Pos: start, Matched: 0, Total: len(pattern)}

	for _, level := range []matchLevel{matchExact, matchTrimEnd, matchTrimAll} {
		limit := max(len(fileLines), 1)
		for i := start; i < limit; i++ {
			matched := 0
			for j := range len(pattern) {
				fi := i + j
				if fi >= len(fileLines) {
					break
				}
				if !linesEqual(fileLines[fi], pattern[j], level) {
					break
				}
				matched++
			}
			if matched > best.Matched {
				best.Pos = i
				best.Matched = matched
				if matched == len(pattern) {
					return best // full match; shouldn't happen but be safe
				}
			}
		}
	}

	// Fill in the divergence details.
	divergeIdx := best.Pos + best.Matched
	best.ExpectedLine = pattern[best.Matched]
	if divergeIdx < len(fileLines) {
		best.ActualLine = fileLines[divergeIdx]
	} else {
		best.PastEOF = true
	}

	return best
}

func seekAt(fileLines, pattern []string, start int, level matchLevel) int {
	limit := len(fileLines) - len(pattern) + 1
	for i := start; i < limit; i++ {
		if matchesAt(fileLines, pattern, i, level) {
			return i
		}
	}
	return -1
}

func matchesAt(fileLines, pattern []string, offset int, level matchLevel) bool {
	for j, p := range pattern {
		if !linesEqual(fileLines[offset+j], p, level) {
			return false
		}
	}
	return true
}
