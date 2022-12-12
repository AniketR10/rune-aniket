package vi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
)

func TestEditorDispatchFocus(t *testing.T) {
	ed := Editor()
	content := "Clement"
	uri, err := workspace.ParseURI("file:///Jolie")
	require.NoError(t, err)

	var h text.Handler
	ed.SubscribeEditorEvents([]textapi.EventType{textapi.EventTypeOpen},
		text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
			assert.Equal(t, textapi.EventTypeOpen, ev.Type)
			assert.Equal(t, content, ev.Content)
			assert.Equal(t, uri, ev.URI)
			h = ev.Resource
			return false
		}))

	var focusCalled int
	ed.SubscribeEditorEvents([]textapi.EventType{textapi.EventTypeFocus},
		text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
			focusCalled++
			assert.Equal(t, textapi.EventTypeFocus, ev.Type)
			assert.Equal(t, uri, ev.URI)
			assert.Equal(t, h, ev.Resource)
			return false
		}))

	buf := cell.NewBuffer()
	buf.WriteString(content)
	_, err = ed.Edit(uri, buf)
	require.NoError(t, err)

	assert.Equal(t, 1, focusCalled)
}

func TestEditorDispatchScroll(t *testing.T) {
	ed := Editor()
	buf := cell.NewBuffer()
	buf.WriteString("Daworg\nSurinach")
	h, err := ed.Edit(workspace.URI{}, buf)
	require.NoError(t, err)

	h.Resize(2, 2)
	h.Handle(term.Event{Type: term.EventKey, Ch: 'j'})

	at := term.Coordinates{X: -1}
	ed.SubscribeEditorEvents([]textapi.EventType{textapi.EventTypeScroll},
		text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
			at = ev.Start
			return false
		}))

	h.Handle(term.Event{Type: term.EventKey, Ch: 'k'})
	assert.Equal(t, term.Coordinates{}, at)

	at = term.Coordinates{X: -1}
	h.Handle(term.Event{Type: term.EventKey, Ch: 'j'})
	assert.Equal(t, term.Coordinates{Y: 1}, at)

	at = term.Coordinates{X: -1}
	h.Handle(term.Event{Type: term.EventKey, Ch: 'j'})
	assert.Equal(t, term.Coordinates{X: -1}, at)
}

func TestEditorDispatchCursor(t *testing.T) {
	uri, err := workspace.ParseURI("file:///tmp/zsh.sh")
	require.NoError(t, err)
	ed := Editor()
	buf := cell.NewBuffer()
	buf.WriteString("Matias\nGiordano\n")
	h, err := ed.Edit(uri, buf)
	require.NoError(t, err)

	// should scroll as well, but changes in cursorAtScroll is what we are expecting
	h.Resize(2, 2)

	windowCursor := term.Coordinates{X: -1}
	scrollCursor := term.Coordinates{X: -1}
	ed.SubscribeEditorEvents([]textapi.EventType{textapi.EventTypeCursor},
		text.FuncEventHandler(func(ctx context.Context, ev textapi.Event) bool {
			windowCursor = ev.Start
			scrollCursor = ev.From
			assert.Equal(t, ev.URI.String(), "file:///tmp/zsh.sh")
			assert.Equal(t, ev.Resource, h)
			return false
		}))

	h.Handle(term.Event{Type: term.EventKey, Ch: 'k'})
	assert.Equal(t, term.Coordinates{X: -1}, windowCursor)
	assert.Equal(t, term.Coordinates{X: -1}, scrollCursor)

	h.Handle(term.Event{Type: term.EventKey, Ch: 'j'})
	assert.Equal(t, term.Coordinates{Y: 0}, windowCursor)
	assert.Equal(t, term.Coordinates{Y: 1}, scrollCursor)

	windowCursor = term.Coordinates{X: -1}
	scrollCursor = term.Coordinates{X: -1}
	h.Handle(term.Event{Type: term.EventKey, Ch: 'l'})
	assert.Equal(t, term.Coordinates{X: 1}, windowCursor)
	assert.Equal(t, term.Coordinates{Y: 1, X: 1}, scrollCursor)
}

func TestEditorSetCursor(t *testing.T) {
	uri, err := workspace.ParseURI("file:///tmp/zsh.sh")
	require.NoError(t, err)

	t.Run("does not return error if cursor already at position", func(t *testing.T) {
		ed := Editor()
		h, err := ed.Edit(uri, cell.NewBuffer())
		require.NoError(t, err)

		err = ed.SetCursor(h, term.Coordinates{})
		require.NoError(t, err)
	})
	t.Run("sets cursor at position", func(t *testing.T) {
		buf := cell.NewBuffer()
		buf.WriteString("a")
		ed := Editor()
		h, err := ed.Edit(uri, buf)
		require.NoError(t, err)

		err = ed.SetCursor(h, term.Coordinates{X: 1})
		require.NoError(t, err)
		pos, err := ed.Cursor(h)
		require.NoError(t, err)
		assert.Equal(t, term.Coordinates{X: 1}, pos)
	})
}
