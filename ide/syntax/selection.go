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
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// SelectionExpand returns the next larger syntactic selection range for rng.
func (t *Tree) SelectionExpand(rng term.Range) (term.Range, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.ready || t.closed || t.tree == nil {
		return term.Range{}, false
	}

	current, ok := t.normalizeSelectionRange(rng)
	if !ok {
		return term.Range{}, false
	}

	startRow, startCol, ok := t.cellBytes.CoordinatesToRunePos(current.Start)
	if !ok {
		return term.Range{}, false
	}
	endRow, endCol, ok := t.cellBytes.CoordinatesToRunePos(current.End)
	if !ok {
		return term.Range{}, false
	}

	start := tree_sitter.Point{Row: uint(startRow), Column: uint(startCol)}
	end := tree_sitter.Point{Row: uint(endRow), Column: uint(endCol)}
	root := t.tree.RootNode()
	node := root.NamedDescendantForPointRange(start, end)
	for node != nil {
		next, ok := t.treeSitterRangeToTerm(node.Range())
		if ok && next != current {
			if containsRange(next, current) {
				return next, true
			}
		}
		node = node.Parent()
	}

	return term.Range{}, false
}

// SelectionShrink returns the next smaller syntactic selection range inside rng.
func (t *Tree) SelectionShrink(rng term.Range, caret term.Coordinates) (term.Range, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.ready || t.closed || t.tree == nil {
		return term.Range{}, false
	}

	current, ok := t.normalizeSelectionRange(rng)
	if !ok || current.Start == current.End {
		return term.Range{}, false
	}
	caret, ok = t.clampSelectionCoordinates(caret)
	if !ok {
		return term.Range{}, false
	}

	caretRow, caretCol, ok := t.cellBytes.CoordinatesToRunePos(caret)
	if !ok {
		return term.Range{}, false
	}
	point := tree_sitter.Point{Row: uint(caretRow), Column: uint(caretCol)}
	node := t.tree.RootNode().NamedDescendantForPointRange(point, point)

	var found term.Range
	foundOK := false
	for node != nil {
		next, ok := t.treeSitterRangeToTerm(node.Range())
		if ok && next != current && containsRange(current, next) {
			found = next
			foundOK = true
		}
		node = node.Parent()
	}

	return found, foundOK
}

func (t *Tree) normalizeSelectionRange(rng term.Range) (term.Range, bool) {
	start, ok := t.clampSelectionCoordinates(rng.Start)
	if !ok {
		return term.Range{}, false
	}
	end, ok := t.clampSelectionCoordinates(rng.End)
	if !ok {
		return term.Range{}, false
	}
	start, end = term.CoordinatesSort(start, end)
	return term.Range{Start: start, End: end}, true
}

func (t *Tree) clampSelectionCoordinates(pos term.Coordinates) (term.Coordinates, bool) {
	rows := t.buf.Rows()
	if rows == 0 || pos.Y < 0 || pos.Y > rows {
		return term.Coordinates{}, false
	}
	if pos.Y == rows {
		return term.Coordinates{Y: pos.Y}, true
	}
	pos.X = max(0, min(pos.X, t.buf.Columns(pos.Y)))
	return pos, true
}

func containsRange(outer, inner term.Range) bool {
	return coordinatesLessOrEqual(outer.Start, inner.Start) &&
		coordinatesLessOrEqual(inner.End, outer.End)
}

func coordinatesLessOrEqual(a, b term.Coordinates) bool {
	if a.Y != b.Y {
		return a.Y < b.Y
	}
	return a.X <= b.X
}
