// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package vctrl

import (
	"context"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
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
	dmp := diffmatchpatch.New()
	dmp.DiffTimeout = timeout
	wSrc, wDst, warray := dmp.DiffLinesToRunes(src, dst)
	diffs = dmp.DiffMainRunes(wSrc, wDst, false)
	diffs = dmp.DiffCharsToLines(diffs, warray)
	return diffs
}

// ApplyDiff applies the given changes to the given editor.
func ApplyDiff(ctx context.Context, ed cell.Editor, changes []diffmatchpatch.Diff) {
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
