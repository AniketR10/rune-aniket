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
	"unicode"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
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

// inlineAttachmentLocationID names the location list that styles the
// still-linked inline attachment labels in the compose buffer.
const inlineAttachmentLocationID = "rune-agent-inline-attachment"

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
	// ReplaceBeforeCursor replaces the n cells immediately preceding the
	// cursor with s. When key is non-zero and s is not empty, the
	// inserted range is linked to that attachment.
	ReplaceBeforeCursor(n int, s string, key AttachmentKey)
	// Links returns the still-valid inline links, ordered by position,
	// as byte offsets into Text().
	Links() []InlineAttachmentLink
	// Unlink drops every inline link to key. The text stays as it is:
	// removing an attachment does not rewrite what the user wrote.
	Unlink(key AttachmentKey)
	// SetDraft replaces the composed text and its inline links. Links
	// that do not fit text are dropped.
	SetDraft(text string, links []InlineAttachmentLink)
	// AtWordBoundary reports whether a token may begin at the cursor.
	AtWordBoundary() bool
	// EnterSubmits handles an <Enter> / <Shift-Enter> key event and
	// reports whether the dialogue should submit the composed message.
	// A bare <Enter> always submits; <Shift-Enter> inserts a newline.
	// When it returns false the event has been consumed by the input as
	// a newline insertion; when it returns true the caller should
	// submit. The event is forwarded to the underlying handler as
	// needed, so callers must not forward it again.
	EnterSubmits(ev term.Event) bool
}

// textHandlerInput adapts a text.Editor handler (and its backing
// buffer) to Input. Text mutations go through the cell editor so a
// modal handler does not reinterpret replayed key events.
type textHandlerInput struct {
	text.Handler
	buf     *cell.Buffer
	tracker *linkTracker
	attr    term.Attributes
	// styled reports whether a location list is currently installed, so
	// an input without links does not clear a list on every keystroke.
	styled       bool
	unsubscribed bool
}

// newTextHandlerInput subscribes to the root publisher of buf, not to
// its usage publisher, so undo and redo reach the range tracker too.
func newTextHandlerInput(
	h text.Handler, buf *cell.Buffer, attr term.Attributes,
) *textHandlerInput {
	in := &textHandlerInput{
		Handler: h,
		buf:     buf,
		attr:    attr,
		tracker: &linkTracker{view: buf},
	}
	buf.Subscribe(in.tracker)
	return in
}

// Handle refreshes the inline link styling once the edit the event may
// have caused has returned. The tracker cannot do this itself: a
// cell.Subscriber must not touch the editor it observes.
func (b *textHandlerInput) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = b.Handler.Handle(ev)
	b.refreshLinks()
	return
}

// Close stops tracking edits. It is idempotent, as the browser handler
// contract requires.
func (b *textHandlerInput) Close() error {
	if !b.unsubscribed {
		b.buf.Unsubscribe(b.tracker)
		b.unsubscribed = true
	}
	return b.Handler.Close()
}

// refreshLinks drops ranges that no longer cover their label and
// re-installs the location list. The editor drops location lists on
// every buffer update, so the list must be rebuilt after each edit.
func (b *textHandlerInput) refreshLinks() {
	b.tracker.prune(b.buf.String())
	if len(b.tracker.links) == 0 {
		if b.styled {
			b.Handler.SetLocationList(
				textapi.LocationPriorityInfo, inlineAttachmentLocationID, nil)
			b.styled = false
		}
		return
	}
	locs := make([]textapi.Location, 0, len(b.tracker.links))
	for _, l := range b.tracker.links {
		locs = append(locs, textapi.Location{
			From: textCoordinates(b.buf, l.start),
			To:   textCoordinates(b.buf, l.end),
			Attr: b.attr,
		})
	}
	b.Handler.SetLocationList(textapi.LocationPriorityInfo,
		inlineAttachmentLocationID, text.LocationSlice(locs))
	b.styled = true
}

// EnterSubmits decides newline-vs-submit for the configured editor.
// A bare <Enter> always submits, regardless of modal (vi) mode;
// <Shift-Enter> inserts a newline. This holds for both modal and
// modeless editors.
func (b *textHandlerInput) EnterSubmits(ev term.Event) bool {
	if ev.Mod&term.ModShift != 0 {
		ev.Mod &^= term.ModShift
		b.Handle(ev)
		return false
	}
	return true
}

func (b *textHandlerInput) Text() string {
	return strings.TrimRight(b.buf.String(), "\n")
}

func (b *textHandlerInput) SetText(s string) {
	b.Handler.CellEditor().Edit(context.Background(), term.Coordinates{}, b.bufEnd(), s)
	b.tracker.reset()
	b.refreshLinks()
}

func (b *textHandlerInput) Clear() {
	b.SetText("")
}

func (b *textHandlerInput) ReplaceBeforeCursor(n int, s string, key AttachmentKey) {
	if n <= 0 && s == "" {
		return
	}
	end := b.Handler.CursorAtScroll()
	start := end
	start.X -= n
	if start.X < 0 {
		start.X = 0
	}
	from, to, _ := b.Handler.CellEditor().Edit(context.Background(), start, end, s)
	b.tracker.add(key, s, textOffset(b.buf, from), textOffset(b.buf, to))
	b.refreshLinks()
}

func (b *textHandlerInput) Links() []InlineAttachmentLink {
	b.tracker.prune(b.buf.String())
	return b.tracker.snapshot()
}

func (b *textHandlerInput) Unlink(key AttachmentKey) {
	b.tracker.unlink(key)
	b.refreshLinks()
}

func (b *textHandlerInput) SetDraft(s string, links []InlineAttachmentLink) {
	b.SetText(s)
	for _, l := range links {
		if l.Start < 0 || l.End > len(s) || l.Start >= l.End {
			continue
		}
		b.tracker.add(l.Key, s[l.Start:l.End], l.Start, l.End)
	}
	b.refreshLinks()
}

func (b *textHandlerInput) AtWordBoundary() bool {
	pos := b.Handler.CursorAtScroll()
	if pos.X == 0 {
		return true
	}
	rows := b.buf.RawCells()
	if pos.Y < 0 || pos.Y >= len(rows) || pos.X > len(rows[pos.Y]) {
		return false
	}
	return unicode.IsSpace(rows[pos.Y][pos.X-1].Ch)
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
