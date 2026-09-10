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

package dialoguetui

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

type recordingOpener struct{ opened []Attachment }

func (r *recordingOpener) OpenAttachment(a Attachment) {
	r.opened = append(r.opened, a)
}

func newChipHandler(t *testing.T) (tui.Handler, *Component, *recordingOpener) {
	t.Helper()
	comp := NewComponent(ComponentConfig{})
	opener := &recordingOpener{}
	h, tx, rx := Handler(context.Background(), new(sync.Mutex), comp,
		term.NopInterrupter(), WithAttachmentOpener(opener))
	t.Cleanup(func() { close(tx) })
	go func() {
		for range rx {
		}
	}()
	h.Resize(40, 14)
	return h, comp, opener
}

func TestAttachmentGridLayoutWraps(t *testing.T) {
	g := newAttachmentGrid([]Attachment{
		NewWorkspaceFileAttachment("alpha.go"),
		NewWorkspaceFileAttachment("bravo.go"),
		NewSymbolAttachment("charlie"),
	}, term.Attributes{})

	assert.Equal(t, 1, g.Height(80), "all chips fit on one row")
	assert.Equal(t, 3, g.Height(14), "each chip needs its own row")

	g.layout(80)
	idx, ok := g.chipAt(term.Coordinates{X: g.slots[1].x})
	require.True(t, ok)
	assert.Equal(t, 1, idx)
	_, ok = g.chipAt(term.Coordinates{X: g.slots[2].x + g.slots[2].width, Y: 0})
	assert.False(t, ok, "the gap after the last chip is not clickable")
}

func TestSendMessageRendersAttachmentChips(t *testing.T) {
	_, comp, _ := newChipHandler(t)
	comp.AddSendMessageAttachments("look at this",
		[]Attachment{NewWorkspaceFileAttachment("alpha.go")})

	w := term.NewStringWriter(40, 14)
	comp.Resize(40, 14)
	comp.Draw(w)
	require.NoError(t, w.Flush())

	out := w.String()
	assert.Contains(t, out, "alpha.go")
	assert.Contains(t, out, "look at this")
	assert.Less(t, strings.Index(out, "alpha.go"), strings.Index(out, "look at this"),
		"chips render above the message body")
}

func TestSentAttachmentClickOpensPreview(t *testing.T) {
	h, comp, opener := newChipHandler(t)
	comp.AddSendMessageAttachments("look at this",
		[]Attachment{NewWorkspaceFileAttachment("alpha.go")})
	h.Draw(term.NewStringWriter(40, 14))

	pos := chipScreenPos(t, comp)
	_, handled := h.Handle(term.Event{
		Type: term.EventMouse, Key: term.MouseLeft,
		MouseX: pos.X, MouseY: pos.Y,
	})

	assert.True(t, handled)
	require.Len(t, opener.opened, 1)
	assert.Equal(t, "alpha.go", opener.opened[0].Path)
}

func TestSentAttachmentHoverHighlights(t *testing.T) {
	h, comp, _ := newChipHandler(t)
	comp.AddSendMessageAttachments("look at this",
		[]Attachment{NewWorkspaceFileAttachment("alpha.go")})
	h.Draw(term.NewStringWriter(40, 14))

	pos := chipScreenPos(t, comp)
	// Key 0 is a bare motion event, which only the GUI backend emits.
	_, _ = h.Handle(term.Event{
		Type: term.EventMouse, MouseX: pos.X, MouseY: pos.Y})
	assert.Equal(t, 0, comp.sentAttachments[0].grid.hovered)

	_, _ = h.Handle(term.Event{
		Type: term.EventMouse, MouseX: pos.X, MouseY: pos.Y + 1})
	assert.Equal(t, -1, comp.sentAttachments[0].grid.hovered,
		"moving off the chip clears the highlight")
}

// chipScreenPos maps the first chip of the first sent grid into the
// screen coordinates a mouse event carries.
func chipScreenPos(t *testing.T, comp *Component) term.Coordinates {
	t.Helper()
	require.Len(t, comp.sentAttachments, 1)
	e := comp.sentAttachments[0]
	require.NotEmpty(t, e.grid.slots)
	return term.CoordinatesSum(
		term.CoordinatesSum(comp.MessagesPosition(), e.origin()),
		term.Coordinates{X: e.grid.slots[0].x, Y: e.grid.slots[0].y})
}
