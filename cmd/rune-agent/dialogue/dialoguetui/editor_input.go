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

package dialoguetui

import (
	"context"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/text"
)

// dialogueComposeURI is the synthetic resource opened by a compose
// editor. It never maps to a real file; the buffer is owned by the
// dialogue component.
var dialogueComposeURI = mustParseURI("dialogue-compose://session")

func mustParseURI(s string) workspaceapi.URI {
	uri, err := workspaceapi.ParseURI(s)
	if err != nil {
		panic(err)
	}
	return uri
}

// Input abstracts the main compose input, backed by a text.Editor
// handler. It is a tui.Handler (so it can be wrapped in a
// handler.Frame) plus the text-mutation helpers the dialogue handler
// relies on.
type Input interface {
	tui.Handler
	Height(width int) int
	Text() string
	SetText(string)
	Clear()
	// EnterSubmits handles an <Enter> / <Shift-Enter> key event and
	// reports whether the dialogue should submit the composed message.
	// When it returns false the event has been consumed by the input as
	// a newline insertion (or ignored); when it returns true the caller
	// should submit. The event is forwarded to the underlying handler as
	// needed, so callers must not forward it again.
	EnterSubmits(ev term.Event) bool
}

// textHandlerInput adapts a text.Editor handler (and its backing
// buffer) to Input. Text mutations go through the cell editor so a
// modal handler does not reinterpret replayed key events.
type textHandlerInput struct {
	text.Handler
	buf *cell.Buffer
	// modal reports whether Handler is a modal (vi-style) editor. For
	// modal editors the handler's own handled signal separates
	// normal-mode <Enter> (unhandled → submit) from insert-mode <Enter>
	// (handled → newline). For modeless editors <Enter> always inserts a
	// newline, so submission is gated on the absence of Shift instead.
	modal bool
}

// EnterSubmits decides newline-vs-submit for the configured editor.
//
// modeless: a bare <Enter> submits; <Shift-Enter> inserts a newline.
// modal: forward <Enter> to the handler; insert mode consumes it as a
// newline (handled), normal mode leaves it unhandled (submit).
func (b *textHandlerInput) EnterSubmits(ev term.Event) bool {
	if !b.modal {
		if ev.Mod&term.ModShift != 0 {
			ev.Mod &^= term.ModShift
			b.Handler.Handle(ev)
			return false
		}
		return true
	}
	ev.Mod &^= term.ModShift
	_, handled := b.Handler.Handle(ev)
	return !handled
}

func (b *textHandlerInput) Text() string {
	return strings.TrimRight(b.buf.String(), "\n")
}

func (b *textHandlerInput) SetText(s string) {
	b.Handler.CellEditor().Edit(context.Background(), term.Coordinates{}, b.bufEnd(), s)
}

func (b *textHandlerInput) Clear() {
	b.SetText("")
}

// Resize re-clamps the editor's scroll offset to the grown viewport.
// The editor only seeks to keep the cursor visible on cursor moves, not
// on Resize, and it never scrolls up to fill empty space at the bottom.
// So a line that wrapped while the box was one row tall leaves the box
// scrolled down by the rows it later grew, hiding earlier lines. Seeking
// up until the offset no longer exceeds the maximum for the new height
// reveals them, while leaving the offset untouched when the content
// still overflows (MaxSeekOffset stays positive and keeps the cursor
// visible).
//
// SeekUp moves the viewport without moving the cursor, whose position is
// stored in window coordinates; left alone it would drift in the buffer.
// Restoring the captured buffer position keeps the cursor pinned to the
// text it was on.
func (b *textHandlerInput) Resize(width, height int) {
	cursor := b.Handler.CursorAtScroll()
	b.Handler.Resize(width, height)
	for b.Handler.SeekOffset() > b.Handler.MaxSeekOffset() && b.Handler.SeekUp() {
	}
	b.Handler.SetCursorAtScroll(cursor)
}

// Height reports the compose box height as the number of visual lines
// the buffer occupies once wrapped at width, so the input row grows as
// the user types past the edge. Counting raw buffer rows would ignore
// wrapping and clip overflowing text.
func (b *textHandlerInput) Height(width int) int {
	if width <= 0 {
		return 0
	}
	rows := b.buf.Rows()
	if rows < 1 {
		return 1
	}
	lines := 0
	for y := range rows {
		cols := b.buf.Columns(y)
		if cols <= 0 {
			lines++
			continue
		}
		lines += (cols + width - 1) / width
	}
	return lines
}

func (b *textHandlerInput) bufEnd() term.Coordinates {
	rows := b.buf.Rows()
	if rows == 0 {
		return term.Coordinates{}
	}
	return term.Coordinates{Y: rows - 1, X: b.buf.Columns(rows - 1)}
}
