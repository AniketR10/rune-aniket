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

package syntax

import (
	"slices"

	log "github.com/sirupsen/logrus"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// CommentCoverage returns exact comment-node ranges when rng is fully covered by
// comment captures from the active highlights query.
func (t *Tree) CommentCoverage(rng term.Range) ([]term.Range, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.ready || t.closed || t.tree == nil || t.highlights == nil {
		return nil, false
	}

	current, ok := t.normalizeSelectionRange(rng)
	if !ok {
		return nil, false
	}

	commentIdx, ok := t.highlights.CaptureIndexForName("comment")
	if !ok {
		return nil, false
	}

	root := t.tree.RootNode()
	cur := tree_sitter.NewQueryCursor()
	defer cur.Close()

	matches := cur.Matches(t.highlights, root, []byte(t.buf.String()))
	var ranges []term.Range
	for {
		m, ok := matches.Next()
		if !ok {
			break
		}
		for _, cap := range m.Captures {
			if uint(cap.Index) != commentIdx {
				continue
			}
			crng, ok := t.treeSitterRangeToTerm(cap.Node.Range())
			if !ok {
				t.log(log.DebugLevel, "could not convert tree sitter range %+v to term",
					cap.Node.Range())
				continue
			}
			if !rangesIntersect(crng, current) {
				continue
			}
			ranges = append(ranges, crng)
		}
	}
	if len(ranges) == 0 {
		return nil, false
	}
	slices.SortFunc(ranges, func(a, b term.Range) int {
		if a.Start.Y != b.Start.Y {
			return a.Start.Y - b.Start.Y
		}
		return a.Start.X - b.Start.X
	})
	if !t.rangesCover(current, ranges) {
		return nil, false
	}
	return ranges, true
}

func rangesIntersect(a, b term.Range) bool {
	return coordinatesLessOrEqual(a.Start, b.End) && coordinatesLessOrEqual(b.Start, a.End)
}

func (t *Tree) rangesCover(target term.Range, ranges []term.Range) bool {
	cursor := target.Start
	for _, rng := range ranges {
		if coordinatesLessOrEqual(rng.End, cursor) {
			continue
		}
		if !coordinatesLessOrEqual(rng.Start, cursor) {
			if !(rng.Start.Y == cursor.Y+1 && rng.Start.X == 0 && cursor.X == t.buf.Columns(cursor.Y)) {
				return false
			}
		}
		if coordinatesLessOrEqual(target.End, rng.End) {
			return true
		}
		cursor = rng.End
	}
	return false
}
