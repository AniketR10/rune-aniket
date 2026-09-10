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
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"unstable.build/rune/internal/text"
)

// saveKill pushes killed text onto the kill ring (the default clipboard
// register). Consecutive kills accumulate into a single entry the way GNU
// kill-append works: forward kills append, backward kills prepend. Entries
// carry StandardSelection metadata so a later C-y inserts the text literally
// at point.
func (h *emacsHandler) saveKill(killed string, prepend bool) {
	if killed == "" {
		return
	}
	entry := killed
	if h.lastKill {
		if prev, err := h.clipboard.Paste(clipboard.DefaultRegisterID); err == nil && prev.Text != "" {
			if prepend {
				entry = killed + prev.Text
			} else {
				entry = prev.Text + killed
			}
		}
	}
	if err := h.clipboard.Copy(clipboard.DefaultRegisterID, clipboard.Data{
		Text:     entry,
		Metadata: text.StandardSelection,
	}); err != nil {
		h.log(log.ErrorLevel, "kill ring copy: %v", err)
	}
	h.killedNow = true
}

// killSelection deletes the active selection and saves the removed text to
// the kill ring.
func (h *emacsHandler) killSelection(prepend bool) bool {
	killed := h.cursor.Selection()
	if killed == "" {
		h.cursor.Unselect()
		return false
	}
	if !h.cursor.DeleteSelection() {
		h.cursor.Unselect()
		return false
	}
	h.saveKill(killed, prepend)
	return true
}

// killLine implements GNU kill-line (C-k): kill from point to the end of
// the line, or the line break itself when point is already at the end of
// the line (an empty line is killed whole). At the very end of the buffer
// there is nothing to kill.
func (h *emacsHandler) killLine() bool {
	if !h.cursor.Select() {
		return false
	}
	h.cursor.MoveEndLine()
	if h.cursor.Selection() != "" {
		return h.killSelection(false)
	}
	h.cursor.Unselect()
	// Point is at the end of the line: kill the newline, joining the next
	// line onto this one. Conflate on an empty line removes the line.
	if h.cursor.CursorAtScroll().Y >= h.cursor.View().Rows()-1 {
		return false
	}
	if !h.cursor.Conflate() {
		return false
	}
	h.saveKill("\n", false)
	return true
}

// killWholeLine implements the C-S-k whole-line kill: the entire line
// including its newline is killed to the kill ring.
func (h *emacsHandler) killWholeLine() bool {
	if !h.cursor.SelectLine() {
		return false
	}
	return h.killSelection(false)
}

// killRegion implements kill-region (C-w): the text between mark and point
// is killed to the kill ring. A kill toward the beginning of the buffer
// (point before mark) prepends to a running kill, like GNU kill-region. A
// shift-selection stands in for an explicit mark, matching GNU
// shift-select-mode where shifted motion activates the region.
func (h *emacsHandler) killRegion() bool {
	point := h.cursor.CursorAtScroll()
	if from, _, ok := h.cursor.SelectionBounds(); ok {
		return h.killSelection(coordLess(point, from))
	}
	loc, ok := h.markLocation()
	if !ok {
		return false
	}
	if !h.cursor.SelectRange(point, loc.From) {
		return false
	}
	if !h.killSelection(coordLess(point, loc.From)) {
		return false
	}
	h.popMarkLocation()
	return true
}
