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

package text

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/cell"
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
