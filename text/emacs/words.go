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

package emacs

import (
	"unicode"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// wordRune reports whether r is a word constituent under the default GNU
// Emacs word syntax: letters and digits. Underscore and other punctuation
// separate words, matching forward-word/backward-word with the stock syntax
// table.
func wordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// docRuneAt returns the rune at pos in document order, exposing the implicit
// line break at end-of-line as '\n'. ok is false only past the end of the
// buffer.
func (h *emacsHandler) docRuneAt(pos term.Coordinates) (rune, bool) {
	view := h.cursor.View()
	rows := view.Rows()
	if pos.Y < 0 || pos.Y >= rows {
		return 0, false
	}
	if pos.X < view.Columns(pos.Y) {
		c, ok := view.Cell(pos)
		if !ok {
			return 0, false
		}
		return c.Ch, true
	}
	if pos.Y == rows-1 {
		return 0, false
	}
	return '\n', true
}

// docAdvance returns the position after pos in document order, stepping onto
// the next line past the end-of-line position.
func (h *emacsHandler) docAdvance(pos term.Coordinates) term.Coordinates {
	if pos.X < h.cursor.View().Columns(pos.Y) {
		pos.X++
		return pos
	}
	return term.Coordinates{Y: pos.Y + 1}
}

// docRetreat returns the position before pos in document order. ok is false
// at the beginning of the buffer.
func (h *emacsHandler) docRetreat(pos term.Coordinates) (term.Coordinates, bool) {
	if pos.X > 0 {
		pos.X--
		return pos, true
	}
	if pos.Y == 0 {
		return pos, false
	}
	return term.Coordinates{Y: pos.Y - 1, X: h.cursor.View().Columns(pos.Y - 1)}, true
}

// forwardWordFrom returns the GNU forward-word target from pos: skip
// characters that are not word constituents (line breaks included), then
// skip the word itself. found reports whether a word was crossed; when no
// word follows, the target is the end of the buffer.
func (h *emacsHandler) forwardWordFrom(pos term.Coordinates) (term.Coordinates, bool) {
	for {
		r, ok := h.docRuneAt(pos)
		if !ok {
			return pos, false
		}
		if wordRune(r) {
			break
		}
		pos = h.docAdvance(pos)
	}
	for {
		r, ok := h.docRuneAt(pos)
		if !ok || !wordRune(r) {
			return pos, true
		}
		pos = h.docAdvance(pos)
	}
}

// backwardWordFrom returns the GNU backward-word target from pos: skip
// non-word characters backward, then move to the start of the word. found
// reports whether a word was crossed; when no word precedes pos, the target
// is the beginning of the buffer.
func (h *emacsHandler) backwardWordFrom(pos term.Coordinates) (term.Coordinates, bool) {
	for {
		prev, ok := h.docRetreat(pos)
		if !ok {
			return pos, false
		}
		if r, _ := h.docRuneAt(prev); wordRune(r) {
			break
		}
		pos = prev
	}
	for {
		prev, ok := h.docRetreat(pos)
		if !ok {
			return pos, true
		}
		if r, _ := h.docRuneAt(prev); !wordRune(r) {
			return pos, true
		}
		pos = prev
	}
}

// moveForwardWord implements forward-word (M-f): move to the end of the
// current or next word, crossing line boundaries. When no word follows,
// point moves to the end of the buffer, as in GNU Emacs.
func (h *emacsHandler) moveForwardWord() bool {
	cur := h.cursor.CursorAtScroll()
	target, _ := h.forwardWordFrom(cur)
	if target == cur {
		return false
	}
	_, ok := h.cursor.MoveToScroll(target)
	return ok
}

// moveBackwardWord implements backward-word (M-b): move to the start of the
// current or previous word, crossing line boundaries. When no word precedes
// point, it moves to the beginning of the buffer, as in GNU Emacs.
func (h *emacsHandler) moveBackwardWord() bool {
	cur := h.cursor.CursorAtScroll()
	target, _ := h.backwardWordFrom(cur)
	if target == cur {
		return false
	}
	_, ok := h.cursor.MoveToScroll(target)
	return ok
}

// killForwardWord implements kill-word (M-d / M-Delete): kill from point to
// the forward-word target.
func (h *emacsHandler) killForwardWord() bool {
	cur := h.cursor.CursorAtScroll()
	target, _ := h.forwardWordFrom(cur)
	if target == cur {
		return false
	}
	if !h.cursor.SelectRange(cur, target) {
		return false
	}
	return h.killSelection(false)
}

// killBackwardWord implements backward-kill-word (M-DEL): kill from the
// backward-word target to point. Consecutive backward kills prepend to the
// running kill-ring entry.
func (h *emacsHandler) killBackwardWord() bool {
	cur := h.cursor.CursorAtScroll()
	target, _ := h.backwardWordFrom(cur)
	if target == cur {
		return false
	}
	if !h.cursor.SelectRange(target, cur) {
		return false
	}
	return h.killSelection(true)
}

// selectWordForward selects from point to the forward-word target, matching
// how the Emacs word-case commands operate on the word at or after point.
func (h *emacsHandler) selectWordForward() bool {
	if _, ok := h.cursor.SelectionMode(); ok {
		return true
	}
	cur := h.cursor.CursorAtScroll()
	target, _ := h.forwardWordFrom(cur)
	if target == cur {
		return false
	}
	if !h.cursor.Select() {
		return false
	}
	h.cursor.MoveToScroll(target)
	return true
}

// rangeText returns the text between from and to without leaving a
// selection behind.
func (h *emacsHandler) rangeText(from, to term.Coordinates) (string, bool) {
	if from == to {
		return "", true
	}
	if !h.cursor.SelectRange(from, to) {
		return "", false
	}
	text := h.cursor.Selection()
	h.cursor.Unselect()
	return text, true
}

// transposeWords implements GNU transpose-words (M-t) following the
// transpose-subr boundary rules: the word at or after point is interchanged
// with the word before it, leaving point after the moved pair. The
// separator between the two words is preserved verbatim, including line
// breaks. When there is no word before point (point inside or before the
// first word of the buffer) there is nothing to transpose and the command
// is a no-op, matching the GNU error.
func (h *emacsHandler) transposeWords() bool {
	origin := h.cursor.CursorAtScroll()
	end2, _ := h.forwardWordFrom(origin)
	start2, found2 := h.backwardWordFrom(end2)
	start1, found1 := h.backwardWordFrom(start2)
	if !found2 || !found1 || start1 == start2 {
		return false
	}
	end1, _ := h.forwardWordFrom(start1)
	if !coordLess(start1, end1) || !coordLess(start2, end2) || coordLess(start2, end1) {
		return false
	}

	w1, ok := h.rangeText(start1, end1)
	if !ok {
		return false
	}
	sep, ok := h.rangeText(end1, start2)
	if !ok {
		return false
	}
	w2, ok := h.rangeText(start2, end2)
	if !ok {
		return false
	}

	if !h.cursor.SelectRange(start1, end2) {
		h.cursor.MoveToScroll(origin)
		return false
	}
	if !h.cursor.DeleteSelection() {
		h.cursor.Unselect()
		h.cursor.MoveToScroll(origin)
		return false
	}
	h.cursor.InsertString(w2 + sep + w1)
	return true
}
