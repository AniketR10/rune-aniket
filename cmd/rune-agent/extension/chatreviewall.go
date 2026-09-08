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

package extension

import (
	"errors"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"

	"unstable.build/rune/cmd/rune-agent/agent/agentools/applypatch"
	"unstable.build/rune/internal/ide/vctrl"
)

// devNullPath is how a unified diff names the absent side of an added or
// deleted file.
const devNullPath = "/dev/null"

// errNoWorkingChanges is returned when the repository has nothing
// uncommitted to review.
var errNoWorkingChanges = errors.New(
	"the working tree has no changes to review")

// reviewWorkingTree renders the uncommitted state of the repository as
// one ordered diff, in the same shape a conversation review produces so
// both open in the same editable window.
func reviewWorkingTree(diffs []vctrl.FileDiff) (changesReview, error) {
	var (
		b      diffBuilder
		review changesReview
		shown  int
	)
	for _, d := range diffs {
		op, ok := workingTreeOp(d)
		if !ok {
			continue
		}
		shown++
		if shown > 1 {
			b.line(diffmatchpatch.DiffEqual, "")
		}
		review.headers = append(review.headers,
			reviewHeader{row: b.rows, text: fileOpHeader(op)})
		review.snippets = append(review.snippets,
			appendPatchDiff(&b, []applypatch.FileOp{op})...)
	}
	if shown == 0 {
		return changesReview{}, errNoWorkingChanges
	}
	review.diffs = b.diffs
	return review, nil
}

// fileOpHeader is the first row appendPatchDiff renders for op, which
// the review emphasises so file boundaries stay visible while scrolling.
func fileOpHeader(op applypatch.FileOp) string {
	switch op.Type {
	case applypatch.OpAdd:
		return "*** Add File: " + op.Path
	case applypatch.OpDelete:
		return "*** Delete File: " + op.Path
	default:
		return "*** Update File: " + op.Path
	}
}

// workingTreeOp converts a unified file diff into the patch operation
// the review renderer speaks.
func workingTreeOp(d vctrl.FileDiff) (applypatch.FileOp, bool) {
	switch {
	case d.OrigName == devNullPath && d.NewName == devNullPath:
		return applypatch.FileOp{}, false
	case d.OrigName == devNullPath:
		lines := hunkLines(d.Hunks)
		if len(lines) == 0 {
			return applypatch.FileOp{}, false
		}
		return applypatch.FileOp{
			Type: applypatch.OpAdd, Path: d.NewName, Lines: lines,
		}, true
	case d.NewName == devNullPath:
		return applypatch.FileOp{
			Type: applypatch.OpDelete, Path: d.OrigName,
		}, true
	}

	op := applypatch.FileOp{Type: applypatch.OpUpdate, Path: d.OrigName}
	if d.NewName != d.OrigName {
		op.MoveTo = d.NewName
	}
	for _, h := range d.Hunks {
		lines := patchLines(h.Body)
		if len(lines) == 0 {
			continue
		}
		op.Hunks = append(op.Hunks, applypatch.Hunk{
			ContextHint: h.Section, Lines: lines,
		})
	}
	if len(op.Hunks) == 0 && op.MoveTo == "" {
		return applypatch.FileOp{}, false
	}
	return op, true
}

// hunkLines flattens the bodies of an added file's hunks into the file
// body applypatch.OpAdd carries.
func hunkLines(hunks []vctrl.Hunk) []applypatch.Line {
	var ret []applypatch.Line
	for _, h := range hunks {
		ret = append(ret, patchLines(h.Body)...)
	}
	return ret
}

// patchLines splits a unified hunk body into patch lines. The "\ No
// newline at end of file" marker carries no content and is dropped: the
// review shows source, not patch metadata.
func patchLines(body string) []applypatch.Line {
	raw := strings.Split(body, "\n")
	if n := len(raw); n > 0 && raw[n-1] == "" {
		raw = raw[:n-1]
	}
	ret := make([]applypatch.Line, 0, len(raw))
	for _, line := range raw {
		if line == "" {
			ret = append(ret, applypatch.Line{Kind: applypatch.LineContext})
			continue
		}
		var kind applypatch.LineKind
		switch line[0] {
		case '+':
			kind = applypatch.LineAdd
		case '-':
			kind = applypatch.LineRemove
		case ' ':
			kind = applypatch.LineContext
		default:
			continue
		}
		ret = append(ret, applypatch.Line{Kind: kind, Content: line[1:]})
	}
	return ret
}
