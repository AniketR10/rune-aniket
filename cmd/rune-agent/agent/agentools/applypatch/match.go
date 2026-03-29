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
