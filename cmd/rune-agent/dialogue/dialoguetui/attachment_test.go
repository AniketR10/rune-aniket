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
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

func pasteEvents(text string) []term.Event {
	evs := []term.Event{{Type: term.EventPasteStart}}
	for _, ch := range text {
		evs = append(evs, term.Event{Type: term.EventKey, Ch: ch})
	}
	return append(evs, term.Event{Type: term.EventPasteEnd})
}

func paste(t *testing.T, h tui.Handler, text string) {
	t.Helper()
	for _, ev := range pasteEvents(text) {
		_, handled := h.Handle(ev)
		assert.True(t, handled, "paste event %v should be handled", ev)
	}
}

func newAttachmentHandler(t *testing.T) (tui.Handler, *Component, <-chan SubmitMessage) {
	t.Helper()
	comp := NewComponent(ComponentConfig{})
	h, tx, rx := Handler(context.Background(), new(sync.Mutex), comp,
		term.NopInterrupter())
	t.Cleanup(func() { close(tx) })
	h.Resize(40, 12)
	return h, comp, rx
}

func writeTempFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte("data"), 0o600))
	return path
}

func TestPastedFilePaths(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b c.txt")
	require.NoError(t, os.WriteFile(a, []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(b, []byte("x"), 0o600))

	tests := []struct {
		name  string
		text  string
		want  []string
		wantK bool
	}{
		{name: "empty", text: "  \n "},
		{name: "plain text", text: "hello world"},
		{name: "missing file", text: filepath.Join(dir, "nope.txt")},
		{name: "directory", text: dir},
		{name: "single file", text: a, want: []string{a}, wantK: true},
		{
			name: "trailing newline",
			text: a + "\n", want: []string{a}, wantK: true,
		},
		{
			name: "newline separated",
			text: a + "\n" + b, want: []string{a, b}, wantK: true,
		},
		{
			name: "escaped spaces",
			text: a + " " + escapeSpaces(b), want: []string{a, b}, wantK: true,
		},
		{
			name: "quoted paths",
			text: `"` + a + `" "` + b + `"`, want: []string{a, b}, wantK: true,
		},
		{
			name: "one missing among many",
			text: a + "\n" + filepath.Join(dir, "nope.txt"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := pastedFilePaths(tt.text)
			assert.Equal(t, tt.wantK, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}

func escapeSpaces(path string) string {
	var out []rune
	for _, r := range path {
		if r == ' ' {
			out = append(out, '\\')
		}
		out = append(out, r)
	}
	return string(out)
}

func TestHandlerPasteCreatesAttachment(t *testing.T) {
	h, comp, _ := newAttachmentHandler(t)
	path := writeTempFile(t, "shot.png")

	paste(t, h, path)

	require.Len(t, comp.Attachments(), 1)
	assert.Equal(t, path, comp.Attachments()[0].Path)
	assert.Equal(t, "shot.png", comp.Attachments()[0].Name)
	assert.True(t, comp.Attachments()[0].IsImage)
	assert.Empty(t, comp.Input().Text())

	w := term.NewStringWriter(40, 12)
	h.Draw(w)
	require.NoError(t, w.Flush())
	assert.Contains(t, w.String(), "shot.png")
}

func TestHandlerPasteMultipleFiles(t *testing.T) {
	h, comp, _ := newAttachmentHandler(t)
	dir := t.TempDir()
	a := filepath.Join(dir, "a.png")
	b := filepath.Join(dir, "b.go")
	require.NoError(t, os.WriteFile(a, []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(b, []byte("x"), 0o600))

	paste(t, h, a+"\n"+b)

	require.Len(t, comp.Attachments(), 2)
	assert.True(t, comp.Attachments()[0].IsImage)
	assert.False(t, comp.Attachments()[1].IsImage)
	assert.Empty(t, comp.Input().Text())
}

func TestHandlerPasteTextGoesToInput(t *testing.T) {
	h, comp, _ := newAttachmentHandler(t)

	paste(t, h, "just some text")

	assert.Empty(t, comp.Attachments())
	assert.Equal(t, "just some text", comp.Input().Text())
}

func TestHandlerClickRemovesAttachment(t *testing.T) {
	h, comp, _ := newAttachmentHandler(t)
	path := writeTempFile(t, "shot.png")
	paste(t, h, path)

	w := term.NewStringWriter(40, 12)
	h.Draw(w)
	require.NoError(t, w.Flush())

	pos, ok := comp.AttachmentsPosition()
	require.True(t, ok)
	_, handled := h.Handle(term.Event{
		Type:   term.EventMouse,
		Key:    term.MouseLeft,
		MouseX: pos.X + 3,
		MouseY: pos.Y,
	})
	assert.True(t, handled)
	assert.Empty(t, comp.Attachments())
	_, ok = comp.AttachmentsPosition()
	assert.False(t, ok)
}

func TestHandlerSubmitCarriesAttachments(t *testing.T) {
	h, comp, rx := newAttachmentHandler(t)
	path := writeTempFile(t, "shot.png")
	paste(t, h, path)

	typeText(h, "look at this")
	done := make(chan SubmitMessage, 1)
	go func() { done <- <-rx }()
	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	require.True(t, handled)

	msg := <-done
	assert.Equal(t, "look at this", msg.Text)
	require.Len(t, msg.Attachments, 1)
	assert.Equal(t, path, msg.Attachments[0].Path)
	assert.Empty(t, comp.Attachments())
}
