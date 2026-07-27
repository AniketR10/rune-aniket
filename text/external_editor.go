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

package text

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
)

// ExternalEditor wraps a CellEditor to preserve the cursor across external edits.
func ExternalEditor(c *Cursor, ed cell.Editor) cell.Editor {
	return extEditor{c, ed}
}

type extEditor struct {
	c  *Cursor
	ed cell.Editor
}

func (e extEditor) Edit(ctx context.Context, start, end term.Coordinates, new string) (
	from, to term.Coordinates, old string,
) {
	cursor := e.c.CursorAtScroll()
	from, to, old = e.ed.Edit(ctx, start, end, new)

	edited := old != "" || from != to
	if !edited {
		return
	}

	if old != "" {
		cursor = cursorAfterExternalDelete(cursor, from, deletedRangeEnd(from, old))
	}
	if from != to {
		cursor = cursorAfterExternalInsert(cursor, from, to)
	}
	e.c.scroll.RecalculateWraps()
	e.c.SetCursorAtScroll(clampExternalCursor(cursor, e.c.View()))
	return
}

func cursorAfterExternalDelete(cursor, from, to term.Coordinates) term.Coordinates {
	switch {
	case coordinatesBefore(cursor, from):
		return cursor
	case coordinatesBefore(cursor, to):
		return from
	case cursor.Y == to.Y:
		return term.Coordinates{Y: from.Y, X: from.X + cursor.X - to.X}
	default:
		cursor.Y -= to.Y - from.Y
		return cursor
	}
}

func cursorAfterExternalInsert(cursor, from, to term.Coordinates) term.Coordinates {
	if coordinatesBefore(cursor, from) {
		return cursor
	}
	if cursor.Y != from.Y {
		cursor.Y += to.Y - from.Y
		return cursor
	}
	if from.Y == to.Y {
		cursor.X += to.X - from.X
		return cursor
	}
	return to
}

func coordinatesBefore(a, b term.Coordinates) bool {
	return a.Y < b.Y || a.Y == b.Y && a.X < b.X
}

func deletedRangeEnd(from term.Coordinates, old string) term.Coordinates {
	rows := term.StringToCells(old)
	if len(rows) <= 1 {
		return term.Coordinates{Y: from.Y, X: from.X + len(rows[0])}
	}
	return term.Coordinates{Y: from.Y + len(rows) - 1, X: len(rows[len(rows)-1])}
}

func clampExternalCursor(cursor term.Coordinates, view cell.View) term.Coordinates {
	if view.Rows() == 0 {
		return term.Coordinates{}
	}
	cursor.Y = max(0, min(cursor.Y, view.Rows()-1))
	cursor.X = max(0, min(cursor.X, view.Columns(cursor.Y)))
	return cursor
}
