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

package extension

import (
	"errors"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"

	"unstable.build/go-tui/cmd/rune-agent/agent/agentools/applypatch"
	"unstable.build/go-tui/ide/vctrl"
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
