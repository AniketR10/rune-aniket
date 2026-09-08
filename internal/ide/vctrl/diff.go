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

package vctrl

import (
	"context"
	"strings"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/rune/internal/cell"
	"unstable.build/rune/internal/component"
)

// Diff computes the (line oriented) modifications needed to turn the src
// string into the dst string. If the given context has a deadline, then this
// is used as a timeout for DiffWithTimeout, otherwise a default sane value
// is used.
func Diff(ctx context.Context, src, dst string) (diffs []diffmatchpatch.Diff) {
	deadline, ok := ctx.Deadline()
	if ok {
		return DiffWithTimeout(src, dst, time.Until(deadline))
	}
	return DiffWithTimeout(src, dst, 10*time.Second)
}

// DiffWithTimeout computes the (line oriented) modifications needed to turn the src
// string into the dst string. The `timeout` argument specifies the maximum
// amount of time it is allowed to spend in this function.
func DiffWithTimeout(src, dst string, timeout time.Duration) (diffs []diffmatchpatch.Diff) {
	// taken from diffmatchpatch.New, but without alloc
	dmp := diffmatchpatch.DiffMatchPatch{
		DiffTimeout:          time.Second,
		DiffEditCost:         4,
		MatchThreshold:       0.5,
		MatchDistance:        1000,
		PatchDeleteThreshold: 0.5,
		PatchMargin:          4,
		MatchMaxBits:         32,
	}
	dmp.DiffTimeout = timeout
	wSrc, wDst, warray := dmp.DiffLinesToRunes(src, dst)
	diffs = dmp.DiffMainRunes(wSrc, wDst, false)
	diffs = dmp.DiffCharsToLines(diffs, warray)
	return diffs
}

// ApplyChanges applies the given diff changes to the given editor.
func ApplyChanges(ctx context.Context, ed cell.Editor, changes []diffmatchpatch.Diff) {
	var cursor int
	for _, change := range changes {
		switch change.Type {
		case diffmatchpatch.DiffDelete:
			from := term.Coordinates{Y: cursor}
			to := from
			for _, ch := range change.Text {
				switch ch {
				case '\n':
					to.Y++
					to.X = 0
				default:
					to.X++
				}
			}
			ed.Edit(ctx, from, to, "")
		case diffmatchpatch.DiffInsert:
			at := term.Coordinates{Y: cursor}
			_, until, _ := ed.Edit(ctx, at, at, change.Text)
			cursor = until.Y
		case diffmatchpatch.DiffEqual:
			for _, ch := range change.Text {
				switch ch {
				case '\n':
					cursor++
				default:
				}
			}
		}
	}
}

// ConvertChangesToFileDiff converts the given slice of diff changes into
// a FileDiff.
func ConvertChangesToFileDiff(
	file workspaceapi.URI, changes []diffmatchpatch.Diff,
) (ret FileDiff) {
	ret.OrigName = file.Path()
	ret.NewName = ret.OrigName
	var cursor int
	for _, change := range changes {
		diffs := [1]diffmatchpatch.Diff{change}
		body := DiffString(diffs[:])

		lines := strings.Split(change.Text, "\n")
		linesWithoutLastEmpty := len(lines)
		if len(lines) > 0 {
			if lines[len(lines)-1] == "" {
				linesWithoutLastEmpty--
			}
		}
		switch change.Type {
		case diffmatchpatch.DiffDelete:
			from := cursor + 1
			ret.Hunks = append(ret.Hunks, Hunk{
				OrigStartLine: int32(from),
				OrigLines:     int32(linesWithoutLastEmpty),
				NewStartLine:  int32(from),
				NewLines:      0,
				Body:          body,
			})
		case diffmatchpatch.DiffInsert:
			from := cursor + 1
			ret.Hunks = append(ret.Hunks, Hunk{
				OrigStartLine: int32(from),
				OrigLines:     0,
				NewStartLine:  int32(from),
				NewLines:      int32(linesWithoutLastEmpty),
				Body:          body,
			})
			cursor += linesWithoutLastEmpty
		case diffmatchpatch.DiffEqual:
			cursor += linesWithoutLastEmpty
		}
	}
	return
}

// DiffString converts the given changes into a diff.
func DiffString(diffs []diffmatchpatch.Diff) string {
	return DiffBuffer(diffs).String()
}

// DiffComponent converts the given changes into a string
// component with background and foreground attributes set.
func DiffComponent(diffs []diffmatchpatch.Diff) tui.Component {
	return component.NewScroll(DiffBuffer(diffs))
}

// DiffBuffer converts the given changes into a cell buffer with background
// and foreground attributes set.
func DiffBuffer(diffs []diffmatchpatch.Diff) *cell.Buffer {
	buf := cell.NewBuffer()
	cursor := term.Coordinates{}
	for _, diff := range diffs {
		text := diff.Text
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			lines := strings.Split(text, "\n")
			for i, line := range lines {
				if i < len(lines)-1 {
					_, cursor = buf.InsertStringWithAttr(cursor, "+", addAttr)
					_, cursor = buf.InsertStringWithAttr(cursor, line, addAttr)
					_, cursor = buf.InsertString(cursor, "\n")
				} else if line != "" {
					_, cursor = buf.InsertStringWithAttr(cursor, "+", addAttr)
					_, cursor = buf.InsertStringWithAttr(cursor, line, addAttr)
				}
			}

		case diffmatchpatch.DiffDelete:
			lines := strings.Split(text, "\n")
			for i, line := range lines {
				if i < len(lines)-1 {
					_, cursor = buf.InsertStringWithAttr(cursor, "-", delAttr)
					_, cursor = buf.InsertStringWithAttr(cursor, line, delAttr)
					_, cursor = buf.InsertString(cursor, "\n")
				} else if line != "" {
					_, cursor = buf.InsertStringWithAttr(cursor, "-", delAttr)
					_, cursor = buf.InsertStringWithAttr(cursor, line, delAttr)
				}
			}
		case diffmatchpatch.DiffEqual:
			_, cursor = buf.InsertString(cursor, text)
		}
	}

	return buf
}

// Changed lines are tinted through the background only, leaving the
// foreground free for syntax highlighting on top. The shades are dark
// enough that default-colored text stays readable over them.
var (
	addAttr = term.Attributes{Bg: term.GetColor("darkgreen")}
	delAttr = term.Attributes{Bg: term.GetColor("darkred")}
)
