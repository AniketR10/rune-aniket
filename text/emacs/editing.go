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

import "github.com/unstablebuild/rune-go-sdk/term"

// lineIsBlank reports whether row y contains only blank cells (spaces, tabs
// or NUL fillers).
func (h *emacsHandler) lineIsBlank(y int) bool {
	view := h.cursor.View()
	if y < 0 || y >= view.Rows() {
		return true
	}
	for x := 0; x < view.Columns(y); x++ {
		c, ok := view.Cell(term.Coordinates{Y: y, X: x})
		if !ok {
			continue
		}
		switch c.Ch {
		case ' ', '\t', 0:
		default:
			return false
		}
	}
	return true
}

// backToIndentation implements M-m: move to the first non-blank character
// of the line, or to the end of the line when it is entirely blank, as GNU
// back-to-indentation does.
func (h *emacsHandler) backToIndentation() bool {
	if h.cursor.MoveStartLineNonBlank() {
		return true
	}
	if h.lineIsBlank(h.cursor.CursorAtScroll().Y) {
		return h.cursor.MoveEndLine()
	}
	return false
}

// justOneSpace implements M-SPC: collapse the horizontal whitespace around
// point to exactly one space, leaving point after it.
func (h *emacsHandler) justOneSpace() bool {
	h.cursor.DeleteHorizontalSpace()
	h.cursor.InsertString(" ")
	return true
}

// deleteIndentation implements GNU delete-indentation (M-^): join the
// current line onto the previous one and fix up whitespace at the join —
// exactly one space, unless the join lands at the start or end of the
// joined line or against a bracket (after an opener or before a closer).
// Point is left at the join, before the inserted space. On the first line
// there is no previous line to join, so the command is a no-op.
func (h *emacsHandler) deleteIndentation() bool {
	pos := h.cursor.CursorAtScroll()
	if pos.Y == 0 {
		return false
	}
	if _, ok := h.cursor.MoveToScroll(term.Coordinates{Y: pos.Y - 1}); !ok {
		return false
	}
	h.cursor.MoveEndLine()
	if !h.cursor.Conflate() {
		h.cursor.MoveToScroll(pos)
		return false
	}
	h.cursor.DeleteHorizontalSpace()
	join := h.cursor.CursorAtScroll()
	if join.X == 0 || join.X >= h.cursor.View().Columns(join.Y) {
		return true
	}
	next, _ := h.docRuneAt(join)
	if _, closer := sexpClosers[next]; closer {
		return true
	}
	if before, ok := h.docRetreat(join); ok {
		prev, _ := h.docRuneAt(before)
		if _, opener := sexpOpeners[prev]; opener {
			return true
		}
	}
	h.cursor.InsertString(" ")
	h.cursor.MoveToScroll(join)
	return true
}

// transposeChars implements GNU transpose-chars (C-t), including the
// cross-line cases the in-line cursor primitive cannot express: at the
// start of a line the swap drags the first character onto the end of the
// previous line; on an empty line it pulls the previous line's last
// character down. At the very beginning of the buffer there is nothing to
// transpose.
func (h *emacsHandler) transposeChars() bool {
	pos := h.cursor.CursorAtScroll()
	view := h.cursor.View()
	if pos.Y >= view.Rows() {
		return false
	}
	cols := view.Columns(pos.Y)
	switch {
	case cols == 0 && pos.Y > 0:
		// Empty line: the two preceding characters are the previous line's
		// last character and the line break; the swap pulls that character
		// down onto this line, leaving point after it.
		pcols := view.Columns(pos.Y - 1)
		if pcols == 0 {
			return false
		}
		c, ok := view.Cell(term.Coordinates{Y: pos.Y - 1, X: pcols - 1})
		if !ok {
			return false
		}
		if !h.cursor.SelectRange(
			term.Coordinates{Y: pos.Y - 1, X: pcols - 1},
			term.Coordinates{Y: pos.Y - 1, X: pcols},
		) {
			return false
		}
		if !h.cursor.DeleteSelection() {
			h.cursor.Unselect()
			return false
		}
		h.cursor.MoveToScroll(term.Coordinates{Y: pos.Y})
		h.cursor.InsertString(string(c.Ch))
		return true
	case pos.Y > 0 && ((pos.X == 0 && cols > 0) || (cols == 1 && pos.X >= cols)):
		// Start of a line (swap the line break with the character after
		// it), or a lone character at the end of its line (swap the two
		// preceding characters, the break and the character): both drag
		// that character onto the end of the previous line and leave point
		// at the start of the shortened line.
		c, ok := view.Cell(term.Coordinates{Y: pos.Y})
		if !ok {
			return false
		}
		prevEnd := term.Coordinates{Y: pos.Y - 1, X: view.Columns(pos.Y - 1)}
		if _, ok := h.cursor.MoveToScroll(prevEnd); !ok {
			return false
		}
		h.cursor.InsertString(string(c.Ch))
		if !h.cursor.SelectRange(
			term.Coordinates{Y: pos.Y},
			term.Coordinates{Y: pos.Y, X: 1},
		) {
			return false
		}
		if !h.cursor.DeleteSelection() {
			h.cursor.Unselect()
			return false
		}
		return true
	default:
		return h.cursor.TransposeChars()
	}
}
