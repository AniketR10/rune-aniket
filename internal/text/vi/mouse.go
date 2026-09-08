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

package vi

import (
	"github.com/unstablebuild/rune-go-sdk/mouse"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/text"
)

// wraps text.CursorDelegate to set vi states
type mouseDelegate struct {
	mouse.Delegate
	vi *viHandlerImpl
}

func newDelegate(vi *viHandlerImpl) mouse.Delegate {
	return mouseDelegate{Delegate: text.CursorMouseDelegate(&vi.cursor), vi: vi}
}

func (d mouseDelegate) SetSelectionStart(pos term.Coordinates) {
	if d.vi.mode() == insertMode {
		// vim's mouse=a: reposition and stay in insert. The anchor
		// must follow because insert-mode arrows snap back to it.
		d.vi.cursor.MoveToScroll(d.vi.cursor.ScrollCoordinates(pos))
		d.vi.anchor = d.vi.cursorAtScroll()
		return
	}
	d.Delegate.SetSelectionStart(pos)
	d.vi.setVisualMode()
	d.vi.anchor = d.vi.cursorAtScroll()
	d.vi.markMatchingBrace()
}

func (d mouseDelegate) SetSelectionEnd(pos term.Coordinates) {
	if !isSelectMode(d.vi.mode()) {
		// Drag from insert: the cursor still sits on the pressed
		// cell, so setVisualMode anchors the selection there.
		d.vi.setVisualMode()
		d.vi.anchor = d.vi.cursorAtScroll()
		d.vi.markMatchingBrace()
	}
	d.Delegate.SetSelectionEnd(pos)
}

func (d mouseDelegate) ClearSelection() {
	if isSelectMode(d.vi.mode()) {
		d.vi.setNormalMode()
	}
	d.Delegate.ClearSelection()
}
