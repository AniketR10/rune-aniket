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

package text

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/cell"
)

// Repeater is a helper structure to enable repeating the last
// text insertion or delete in a buffer. See Repeat for more details.
type Repeater struct {
	buf    *cell.Buffer
	cursor *Cursor

	i         bool
	insertStr string

	d          bool
	repeating  bool
	deleteFrom term.Coordinates
	deleteTo   term.Coordinates
}

// NewRepeater allocates storage for a Repeater and initializes it.
func NewRepeater(cursor *Cursor, buf *cell.Buffer) *Repeater {
	r := new(Repeater)
	r.Init(cursor, buf)
	return r
}

// Init initializes this Repeater with buf by subscribing it to it.
func (r *Repeater) Init(cursor *Cursor, buf *cell.Buffer) {
	r.buf = buf
	r.cursor = cursor
	buf.SubscribeUsage(r)
}

// Repeat repeats the last Delete or last text insertion.
//
// Last text insertion is the accumulated string written through buffer's Writer's
// Insert since construction of Repeater or since last call to Repeater's Clear.
func (r *Repeater) Repeat() (ok bool) {
	if r.i {
		// avoid OnWillEdit loop
		r.repeating = true
		r.cursor.InsertString(r.insertStr)
		r.repeating = false
		ok = true
		return
	}
	if r.d {
		cursor := r.cursor.CursorAtScroll()
		from, to := term.CoordinatesSort(r.deleteFrom, r.deleteTo)
		diff := term.CoordinatesDiff(to, from)

		from = cursor
		to = term.CoordinatesSum(from, diff)

		// avoid OnWillEdit loop
		r.repeating = true
		_, str := r.buf.Delete(from, to)
		r.repeating = false

		ok = str != ""
		return
	}
	return
}

// Clear resets the insert string to be repeated.
func (r *Repeater) Clear() {
	r.insertStr = ""
}

// OnWillEdit satisfies cell.Subscriber.
func (r *Repeater) OnWillEdit(
	ctx context.Context, start, end term.Coordinates, str string,
) {
	if r.repeating {
		return
	}
	r.d = r.insertStr == "" && start != end
	r.i = r.insertStr != "" || str != ""
	r.insertStr += str
	r.deleteFrom = start
	r.deleteTo = end
}

// OnDidEdit satisfies cell.Subscriber.
func (r *Repeater) OnDidEdit(
	ctx context.Context, from, to term.Coordinates, old string,
) {
	// text has been deleted most likely by backlash
	if r.insertStr != "" && old != "" {
		idx := len(r.insertStr) - len(old)
		if idx >= 0 && idx < len(r.insertStr) {
			r.insertStr = r.insertStr[:idx]
		}
	}
}
