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
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"unstable.build/go-tui/text"
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
// (point before mark) prepends to a running kill, like GNU kill-region.
func (h *emacsHandler) killRegion() bool {
	loc, ok := h.markLocation()
	if !ok {
		return false
	}
	point := h.cursor.CursorAtScroll()
	if !h.cursor.SelectRange(point, loc.From) {
		return false
	}
	if !h.killSelection(coordLess(point, loc.From)) {
		return false
	}
	h.popMarkLocation()
	return true
}
