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

package emacs

import (
	"github.com/unstablebuild/rune-go-sdk/term"
)

// startQuotedInsert begins C-q (quoted-insert): the next key is inserted as
// a literal character instead of running its command. count repeats the
// inserted character; a non-positive count inserts nothing but still
// consumes the quoted key, like GNU's prefix-numeric-value handling.
func (h *emacsHandler) startQuotedInsert(count int) bool {
	h.pendingQuotedInsert = true
	h.quotedInsertCount = count
	h.setTransientMode("QUOTE")
	h.less.SetMessage("C-q-")
	return true
}

// handleQuotedInsertKey consumes the key read after C-q and inserts the
// character it denotes. Control chords insert their ASCII control code, so
// C-q C-i inserts a tab and C-q C-j inserts a newline, as in GNU. A key
// that denotes no character is dropped.
func (h *emacsHandler) handleQuotedInsertKey(ev term.Event) {
	h.pendingQuotedInsert = false
	h.setTransientMode("")
	h.less.SetMessage("")
	if ev.Type != term.EventKey {
		return
	}
	ch, ok := quotedInsertRune(ev)
	if !ok {
		return
	}
	if _, selected := h.cursor.SelectionMode(); selected {
		h.cursor.DeleteSelection()
		h.cursor.Unselect()
	}
	// One command, one undo: a counted quote reverts in a single step, and
	// it never joins a surrounding run of self-inserted characters.
	h.closeUndoRun()
	h.buf.MarkStartUndo()
	defer h.buf.GroupUndo()
	for i := 0; i < h.quotedInsertCount; i++ {
		h.cursor.Insert(ch)
	}
}

// quotedInsertRune maps a key event to the literal character C-q inserts for
// it. Meta chords have no literal form and are rejected.
func quotedInsertRune(ev term.Event) (rune, bool) {
	if ev.Mod&term.ModAlt != 0 {
		return 0, false
	}
	switch ev.Key {
	case term.KeyEnter:
		return '\n', true
	case term.KeyTab:
		return '\t', true
	case term.KeySpace:
		return ' ', true
	case term.KeyEsc:
		return 0x1b, true
	}
	if ev.Ch == 0 {
		return 0, false
	}
	if ev.Mod&term.ModCtrl != 0 {
		// C-<letter> denotes the matching ASCII control code.
		upper := ev.Ch
		if upper >= 'a' && upper <= 'z' {
			upper -= 'a' - 'A'
		}
		if upper < '@' || upper > '_' {
			return 0, false
		}
		return upper - '@', true
	}
	return ev.Ch, true
}
