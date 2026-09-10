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
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// queryReplaceState drives the interactive M-% loop after both the search and
// replacement strings have been read. Each edit resets the cursor's location
// lists, so the loop re-runs the search and finds the next match at or after
// scanFrom rather than caching indices.
type queryReplaceState struct {
	active      bool
	search      string
	replacement string
	// scanFrom is the position from which the next match is sought. After a
	// replacement it advances past the inserted text so a replacement that
	// contains the search string is not matched again.
	scanFrom term.Coordinates
}

// startQueryReplace begins M-%: it reads the search string, then the
// replacement string, then enters the decision loop.
func (h *emacsHandler) startQueryReplace() bool {
	h.setTransientMode("QUERY")
	h.minibuffer.start("Query replace: ", func(search string) {
		if search == "" {
			h.less.SetMessage("")
			h.setTransientMode("")
			return
		}
		h.minibuffer.start("Query replace "+search+" with: ", func(replacement string) {
			h.beginQueryReplaceLoop(search, replacement)
		}, func() {
			h.less.SetMessage("")
			h.setTransientMode("")
		})
		h.renderPrompt()
	}, func() {
		h.less.SetMessage("")
		h.setTransientMode("")
	})
	h.renderPrompt()
	return true
}

// beginQueryReplaceLoop enters the interactive decision loop, positioning point
// on the first match at or after the caret.
func (h *emacsHandler) beginQueryReplaceLoop(search, replacement string) {
	h.queryReplace = queryReplaceState{
		active:      true,
		search:      search,
		replacement: replacement,
		scanFrom:    h.cursor.CursorAtScroll(),
	}
	// The whole session is one command in GNU terms: every replacement it
	// performs merges into a single undo group when the loop finishes.
	h.closeUndoRun()
	h.buf.MarkStartUndo()
	if !h.queryReplaceSeek() {
		h.finishQueryReplace("Replaced 0 occurrences")
	}
}

// queryReplaceSeek moves point to the next match at or after scanFrom and
// selects it. It returns false when no further match exists.
func (h *emacsHandler) queryReplaceSeek() bool {
	if h.cursor.Search(h.queryReplace.search) == 0 {
		return false
	}
	locs := h.searchLocations()
	idx, ok := nextMatchFrom(locs, h.queryReplace.scanFrom)
	if !ok {
		return false
	}
	loc := locs[idx]
	h.cursor.MoveToScroll(loc.From)
	h.cursor.SelectRange(loc.From, loc.To)
	h.renderQueryReplace()
	return true
}

// nextMatchFrom returns the index of the first match whose start is at or after
// pos. Unlike firstMatchFrom it does not wrap: query-replace runs from point to
// the end of the buffer.
func nextMatchFrom(locs []textapi.Location, pos term.Coordinates) (int, bool) {
	for i, loc := range locs {
		if !coordLess(loc.From, pos) {
			return i, true
		}
	}
	return 0, false
}

// replaceCurrentMatch replaces the selected match with the replacement text and
// advances scanFrom past the inserted text.
func (h *emacsHandler) replaceCurrentMatch() {
	a, b, ok := h.cursor.SelectionBounds()
	if !ok {
		return
	}
	lo, _ := sortBounds(a, b)
	h.cursor.DeleteSelection()
	h.cursor.Unselect()
	h.cursor.MoveToScroll(lo)
	h.cursor.InsertString(h.queryReplace.replacement)
	h.queryReplace.scanFrom = h.cursor.CursorAtScroll()
}

// skipCurrentMatch advances scanFrom past the selected match without editing.
func (h *emacsHandler) skipCurrentMatch() {
	a, b, ok := h.cursor.SelectionBounds()
	if !ok {
		return
	}
	_, hi := sortBounds(a, b)
	h.cursor.Unselect()
	h.queryReplace.scanFrom = hi
}

// sortBounds returns a and b ordered so lo precedes hi in document order.
func sortBounds(a, b term.Coordinates) (lo, hi term.Coordinates) {
	if coordLess(b, a) {
		return b, a
	}
	return a, b
}

// finishQueryReplace ends the loop, clears the highlight and shows msg.
func (h *emacsHandler) finishQueryReplace(msg string) {
	h.buf.GroupUndo()
	h.queryReplace.active = false
	h.cursor.Unselect()
	h.cursor.Search("")
	h.less.SetMessage("%s", msg)
	h.setTransientMode("")
}

// renderQueryReplace shows the y/n/!/q prompt for the pending match.
func (h *emacsHandler) renderQueryReplace() {
	h.less.SetMessage("Query replacing %s with %s (y/n/!/./q)",
		h.queryReplace.search, h.queryReplace.replacement)
}

// handleQueryReplaceKey consumes one key during the decision loop.
func (h *emacsHandler) handleQueryReplaceKey(ev term.Event) {
	if ev.Type != term.EventKey {
		return
	}
	if ev.Mod == term.ModCtrl && ev.Ch == 'g' {
		h.finishQueryReplace("Quit")
		return
	}
	switch ev.Key {
	case term.KeyEsc, term.KeyEnter:
		h.finishQueryReplace("Done")
		return
	case term.KeySpace:
		h.queryReplaceReplaceAndAdvance()
		return
	case term.KeyDelete, term.KeyBackspace:
		h.queryReplaceSkipAndAdvance()
		return
	}
	switch ev.Ch {
	case 'y':
		h.queryReplaceReplaceAndAdvance()
	case 'n':
		h.queryReplaceSkipAndAdvance()
	case '!':
		h.queryReplaceAll()
	case '.':
		h.replaceCurrentMatch()
		h.finishQueryReplace("Done")
	case 'q':
		h.finishQueryReplace("Done")
	}
}

func (h *emacsHandler) queryReplaceReplaceAndAdvance() {
	h.replaceCurrentMatch()
	if !h.queryReplaceSeek() {
		h.finishQueryReplace("Done")
	}
}

func (h *emacsHandler) queryReplaceSkipAndAdvance() {
	h.skipCurrentMatch()
	if !h.queryReplaceSeek() {
		h.finishQueryReplace("Done")
	}
}

// queryReplaceAll replaces the current and every remaining match in one step.
func (h *emacsHandler) queryReplaceAll() {
	for {
		h.replaceCurrentMatch()
		if !h.queryReplaceSeek() {
			h.finishQueryReplace("Done")
			return
		}
	}
}
