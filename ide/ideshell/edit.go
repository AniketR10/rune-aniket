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

package ideshell

import (
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
)

// enterEditMode opens an EditSession seeded from the inner inputbox's
// current text (so the user can keep editing what they had typed at
// the prompt). The cursor is placed at the end of the buffer to
// match the inputbox, where the prompt's caret is anchored.
func (h *Handler) enterEditMode() {
	if h.editSession == nil || h.editSession.Active() {
		return
	}
	buf := cell.NewBuffer()
	buf.WriteString(h.inner.Text())
	h.editBuf = buf
	h.editSession.Resize(h.width, h.editEditorH())
	h.editSession.Begin(buf, term.Coordinates{X: buf.Columns(0)})
	// Cancel any pending search/completion overlay so it doesn't
	// composite behind the editor or accept stray events.
	if h.searching {
		h.cancelSearch()
	}
}

// exitEditMode tears down the EditSession and replaces the inner
// inputbox text with the buffer's final contents. submit forwards
// the original <enter> through to the inner repl so it dispatches
// the (now-replayed) command.
func (h *Handler) exitEditMode(submit bool, ev term.Event) (exit, handled bool) {
	if h.editSession == nil || !h.editSession.Active() {
		return false, true
	}
	final := h.editBuf.String()
	h.editSession.End()
	h.editBuf = nil
	h.replaceInputText(final)
	if submit {
		return h.inner.Handle(ev)
	}
	return false, true
}

// editEditorH returns the number of rows reserved for the editor
// while modal edit mode is active. The editor lives in the same
// vertical band the inner inputbox owns when the shell is idle, so
// the prompt visually stays anchored to the bottom of the screen.
func (h *Handler) editEditorH() int {
	if h.height <= 0 {
		return 0
	}
	rlH, _ := h.inner.LayoutHeights()
	return min(max(rlH, 1), h.height)
}

// editInnerH returns the number of rows allocated to the inner repl
// (its previous output) while modal edit mode is active. The editor
// owns the remaining bottom rows.
func (h *Handler) editInnerH() int {
	return max(0, h.height-h.editEditorH())
}

// drawEdit composites the inner repl's output onto the top portion
// of the writer and the editor onto the bottom band, where the
// inputbox is normally drawn.
func (h *Handler) drawEdit(w term.Writer) {
	innerH := h.editInnerH()
	editorH := h.editEditorH()
	// Render the inner to its own buffer and only copy the
	// top innerH rows so its inputbox row never reaches the
	// real writer (the editor takes its place).
	if innerH > 0 {
		buf := term.NewStringWriter(h.width, h.height)
		h.inner.Draw(buf)
		_, outH := h.inner.LayoutHeights()
		copyH := min(outH, innerH)
		cells := buf.Cells()
		for y := range copyH {
			for x := range h.width {
				w.SetCell(term.Coordinates{X: x, Y: y},
					cells[y*h.width+x])
			}
		}
	}
	editorW := &component.VirtualWriter{
		Writer: w,
		Offset: term.Coordinates{Y: innerH},
		Width:  h.width,
		Height: editorH,
	}
	// Resize keeps the editor session up to date; idempotent.
	h.editSession.Resize(h.width, editorH)
	if drawer, ok := h.editSession.Drawer(); ok {
		drawer.Draw(editorW)
	}
}
