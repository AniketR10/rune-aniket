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
