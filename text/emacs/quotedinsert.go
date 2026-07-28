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
