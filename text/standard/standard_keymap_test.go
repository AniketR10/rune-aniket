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

package standard

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/text"
)

// newStandardKeymapHandler builds a real standard handler over content with an
// in-memory clipboard, positions the cursor, and returns the handler
// plus the backing buffer and clipboard for assertions.
func newStandardKeymapHandler(
	t *testing.T, content string, at term.Coordinates,
) (text.Handler, *cell.Buffer, clipboard.Register) {
	t.Helper()
	uri, err := workspaceapi.ParseURI("test:///")
	require.NoError(t, err)
	buf := cell.NewBuffer()
	buf.ReadFrom(strings.NewReader(content))
	clip := clipboard.NewInMemory()
	h := NewHandler(buf, uri, text.IndentRuneTab, 0,
		WithClipboard(clip),
		WithTabspaces(1),
	)
	h.Resize(80, 20)
	if at.X != 0 || at.Y != 0 {
		require.True(t, h.SetCursorAtScroll(at))
	}
	return h, buf, clip
}

// TestStandardKeymapMotion pins the standard editor's word/line motions
// on ctrl-arrows and ctrl-home/end, replacing the old emacs single-key
// motions.
func TestStandardKeymapMotion(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		at      term.Coordinates
		ev      term.Event
		wantAt  term.Coordinates
	}{
		{
			name:    "ctrl-right moves to end of word",
			content: "foo bar",
			ev:      term.Event{Type: term.EventKey, Mod: term.ModCtrl, Key: term.KeyArrowRight},
			wantAt:  term.Coordinates{X: 3},
		},
		{
			name:    "ctrl-left moves to start of word",
			content: "foo bar",
			at:      term.Coordinates{X: 7},
			ev:      term.Event{Type: term.EventKey, Mod: term.ModCtrl, Key: term.KeyArrowLeft},
			wantAt:  term.Coordinates{X: 4},
		},
		{
			name:    "ctrl-home moves to first line",
			content: "a\nb\nc",
			at:      term.Coordinates{Y: 2},
			ev:      term.Event{Type: term.EventKey, Mod: term.ModCtrl, Key: term.KeyHome},
			wantAt:  term.Coordinates{Y: 0},
		},
		{
			name:    "ctrl-end moves to last line",
			content: "a\nb\nc",
			ev:      term.Event{Type: term.EventKey, Mod: term.ModCtrl, Key: term.KeyEnd},
			wantAt:  term.Coordinates{Y: 2},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, _, _ := newStandardKeymapHandler(t, tc.content, tc.at)
			_, handled := h.Handle(tc.ev)
			assert.True(t, handled, "event must be handled")
			assert.Equal(t, tc.wantAt, h.CursorAtScroll())
		})
	}
}

// TestStandardKeymapSelectAll pins ctrl-a selecting the whole buffer
// instead of the old emacs move-to-line-start.
func TestStandardKeymapSelectAll(t *testing.T) {
	h, _, _ := newStandardKeymapHandler(t, "hello\nworld", term.Coordinates{})
	_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'a'})
	require.True(t, handled)
	sel, ok := h.Selection()
	require.True(t, ok, "ctrl-a must create a selection")
	assert.Equal(t, "hello\nworld\n", sel)
}

// TestStandardKeymapSelectLine pins ctrl-l selecting the current line.
func TestStandardKeymapSelectLine(t *testing.T) {
	h, _, _ := newStandardKeymapHandler(t, "hello\nworld", term.Coordinates{})
	_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'l'})
	require.True(t, handled)
	_, ok := h.Selection()
	assert.True(t, ok, "ctrl-l must select the line")
}

// TestStandardKeymapClipboard pins ctrl-c copy, ctrl-x cut, ctrl-v paste.
func TestStandardKeymapClipboard(t *testing.T) {
	t.Run("ctrl-x cuts current line when no selection", func(t *testing.T) {
		h, buf, clip := newStandardKeymapHandler(t, "one\ntwo\nthree", term.Coordinates{Y: 1})
		_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'x'})
		require.True(t, handled)
		assert.Equal(t, "one\nthree", buf.String())
		data, err := clip.Paste(clipboard.DefaultRegisterID)
		require.NoError(t, err)
		assert.Contains(t, data.Text, "two")
	})

	t.Run("ctrl-c copies selection without deleting", func(t *testing.T) {
		h, buf, clip := newStandardKeymapHandler(t, "one\ntwo\nthree", term.Coordinates{Y: 1})
		_, _ = h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'l'})
		_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'c'})
		require.True(t, handled)
		assert.Equal(t, "one\ntwo\nthree", buf.String())
		data, err := clip.Paste(clipboard.DefaultRegisterID)
		require.NoError(t, err)
		assert.Contains(t, data.Text, "two")
	})

	t.Run("ctrl-v pastes clipboard", func(t *testing.T) {
		h, buf, clip := newStandardKeymapHandler(t, "ab", term.Coordinates{})
		require.NoError(t, clip.Copy(clipboard.DefaultRegisterID,
			clipboard.Data{Text: "X", Metadata: text.StandardSelection}))
		_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'v'})
		require.True(t, handled)
		assert.Equal(t, "Xab", buf.String())
	})
}

// TestStandardKeymapUndoRedo pins ctrl-z undo and ctrl-y / ctrl-shift-z redo.
func TestStandardKeymapUndoRedo(t *testing.T) {
	t.Run("ctrl-z undoes", func(t *testing.T) {
		h, buf, _ := newStandardKeymapHandler(t, "", term.Coordinates{})
		_, _ = h.Handle(term.Event{Type: term.EventKey, Ch: 'a'})
		require.Equal(t, "a", buf.String())
		_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'z'})
		require.True(t, handled)
		assert.Equal(t, "", buf.String())
	})

	t.Run("ctrl-y redoes", func(t *testing.T) {
		h, buf, _ := newStandardKeymapHandler(t, "", term.Coordinates{})
		_, _ = h.Handle(term.Event{Type: term.EventKey, Ch: 'a'})
		_, _ = h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'z'})
		require.Equal(t, "", buf.String())
		_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'y'})
		require.True(t, handled)
		assert.Equal(t, "a", buf.String())
	})

	t.Run("ctrl-shift-z redoes", func(t *testing.T) {
		h, buf, _ := newStandardKeymapHandler(t, "", term.Coordinates{})
		_, _ = h.Handle(term.Event{Type: term.EventKey, Ch: 'a'})
		_, _ = h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'z'})
		_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'Z'})
		require.True(t, handled)
		assert.Equal(t, "a", buf.String())
	})
}

// TestStandardKeymapDeletion pins ctrl-backspace/ctrl-delete word delete
// and ctrl-shift-k line delete.
func TestStandardKeymapDeletion(t *testing.T) {
	t.Run("ctrl-backspace deletes word to the left", func(t *testing.T) {
		h, buf, _ := newStandardKeymapHandler(t, "foo bar", term.Coordinates{X: 7})
		_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Key: term.KeyBackspace})
		require.True(t, handled)
		assert.Equal(t, "foo ", buf.String())
	})

	t.Run("ctrl-delete deletes word to the right", func(t *testing.T) {
		h, buf, _ := newStandardKeymapHandler(t, "foo bar", term.Coordinates{})
		_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Key: term.KeyDelete})
		require.True(t, handled)
		assert.Equal(t, "bar", buf.String())
	})

	t.Run("ctrl-shift-k deletes the line", func(t *testing.T) {
		h, buf, _ := newStandardKeymapHandler(t, "one\ntwo\nthree", term.Coordinates{Y: 1})
		_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'K'})
		require.True(t, handled)
		assert.Equal(t, "one\nthree", buf.String())
	})
}

// TestStandardKeymapLineInsert pins ctrl-enter (below) and
// ctrl-shift-enter (above) line insertion.
func TestStandardKeymapLineInsert(t *testing.T) {
	t.Run("ctrl-enter inserts line below", func(t *testing.T) {
		h, buf, _ := newStandardKeymapHandler(t, "a\nb", term.Coordinates{})
		_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Key: term.KeyEnter})
		require.True(t, handled)
		assert.Equal(t, "a\n\nb", buf.String())
	})

	t.Run("ctrl-shift-enter inserts line above", func(t *testing.T) {
		h, buf, _ := newStandardKeymapHandler(t, "a\nb", term.Coordinates{})
		_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl | term.ModShift, Key: term.KeyEnter})
		require.True(t, handled)
		assert.Equal(t, "\na\nb", buf.String())
	})
}

// TestStandardKeymapConflate pins ctrl-shift-j joining lines.
func TestStandardKeymapConflate(t *testing.T) {
	h, buf, _ := newStandardKeymapHandler(t, "one\ntwo", term.Coordinates{})
	_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'J'})
	require.True(t, handled)
	assert.Equal(t, "onetwo", buf.String())
}

// TestStandardKeymapDropsEmacsBindings asserts the dropped emacs
// single-key motions no longer perform their emacs actions in the
// standard editor.
func TestStandardKeymapDropsEmacsBindings(t *testing.T) {
	t.Run("ctrl-e no longer moves to end of line", func(t *testing.T) {
		h, _, _ := newStandardKeymapHandler(t, "hello", term.Coordinates{})
		_, _ = h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'e'})
		assert.NotEqual(t, term.Coordinates{X: 5}, h.CursorAtScroll(),
			"ctrl-e must not behave as emacs move-end-line")
	})

	t.Run("ctrl-d no longer deletes a single char", func(t *testing.T) {
		h, buf, _ := newStandardKeymapHandler(t, "hello", term.Coordinates{})
		_, _ = h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'd'})
		assert.NotEqual(t, "ello", buf.String(),
			"ctrl-d must not behave as emacs delete-char")
	})

	t.Run("ctrl-b no longer moves left", func(t *testing.T) {
		h, _, _ := newStandardKeymapHandler(t, "hello", term.Coordinates{X: 3})
		_, _ = h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'b'})
		assert.Equal(t, term.Coordinates{X: 3}, h.CursorAtScroll(),
			"ctrl-b must not behave as emacs move-left")
	})
}
