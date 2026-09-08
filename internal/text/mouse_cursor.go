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
	"github.com/unstablebuild/rune-go-sdk/mouse"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/component"
)

// CursorMouseDelegate satisfies mouse.Delegate with a Cursor on a component.Scroll.
func CursorMouseDelegate(c *Cursor) mouse.Delegate {
	return mouseDelegate{cursor: c}
}

// satisfies mouse.Delegate
type mouseDelegate struct {
	cursor *Cursor
}

func (d mouseDelegate) OnAction(ev term.Event, pos term.Coordinates, action mouse.Action) bool {
	return false
}

func (d mouseDelegate) ScrollUp(n int) (ok bool) {
	for range n {
		ok = d.scroll().SeekUp()
		if !ok {
			return
		}
	}
	return
}

func (d mouseDelegate) ScrollDown(n int) (ok bool) {
	for range n {
		ok = d.scroll().SeekDown()
		if !ok {
			return
		}
	}
	return
}

func (d mouseDelegate) SetSelectionStart(pos term.Coordinates) {
	if _, ok := d.cursor.SelectionMode(); ok {
		d.cursor.Unselect()
	}
	pos = d.cursor.ScrollCoordinates(pos)
	d.cursor.MoveToScroll(pos)
	d.cursor.Select()
}

func (d mouseDelegate) SetSelectionEnd(pos term.Coordinates) {
	if _, ok := d.cursor.SelectionMode(); !ok {
		return
	}
	pos = d.cursor.ScrollCoordinates(pos)
	d.cursor.MoveToScroll(pos)
}

func (d mouseDelegate) ClearSelection() {
	d.cursor.Unselect()
}

func (d mouseDelegate) SelectWordAt(pos term.Coordinates) {
	pos = d.cursor.ScrollCoordinates(pos)
	buf := d.scroll().Buffer()
	if pos.Y < 0 || pos.Y >= buf.Rows() {
		return
	}
	start, end, word := d.scroll().WordAt(pos)
	if word == "" {
		return
	}
	// Anchoring at start may scroll the window to reveal it, which
	// would invalidate a window-relative end captured beforehand.
	if _, ok := d.cursor.SelectionMode(); ok {
		d.cursor.Unselect()
	}
	d.cursor.MoveToScroll(start)
	d.cursor.Select()
	d.cursor.MoveToScroll(end)
}

func (d mouseDelegate) SelectLine(y int) {
	if _, ok := d.cursor.SelectionMode(); ok {
		d.cursor.Unselect()
	}
	pos := d.cursor.ScrollCoordinates(term.Coordinates{Y: y})
	d.cursor.MoveToScroll(pos)
	d.cursor.SelectLine()
}

func (d mouseDelegate) Width() int {
	return d.scroll().Width()
}

func (d mouseDelegate) Height() int {
	return d.scroll().SizeHeight()
}

func (d mouseDelegate) scroll() *component.Scroll {
	return d.cursor.scroll
}
