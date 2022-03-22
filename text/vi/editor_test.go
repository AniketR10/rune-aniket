package vi

import (
	"context"
	"testing"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
	"github.com/ernestrc/go-tui/text"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditorDispatchFocus(t *testing.T) {
	ed := Editor()
	content := "Clement"
	name := "Jolie"

	var h text.Handler
	ed.SubscribeEditorEvents([]text.EventType{text.EventTypeOpen},
		text.FuncEventHandler(func(ctx context.Context, ev text.Event) bool {
			assert.Equal(t, text.EventTypeOpen, ev.Type)
			assert.Equal(t, content, ev.Content)
			assert.Equal(t, name, ev.ResourceName)
			h = ev.Resource
			return false
		}))

	var focusCalled int
	ed.SubscribeEditorEvents([]text.EventType{text.EventTypeFocus},
		text.FuncEventHandler(func(ctx context.Context, ev text.Event) bool {
			focusCalled++
			assert.Equal(t, text.EventTypeFocus, ev.Type)
			assert.Equal(t, name, ev.ResourceName)
			assert.Equal(t, h, ev.Resource)
			return false
		}))

	buf := cell.NewBuffer()
	buf.WriteString(content)
	_, err := ed.Edit(name, buf)
	require.NoError(t, err)

	assert.Equal(t, 1, focusCalled)
}

func TestEditorDispatchScroll(t *testing.T) {
	ed := Editor()
	buf := cell.NewBuffer()
	buf.WriteString("Daworg\nSurinach")
	h, err := ed.Edit("oh my...", buf)
	require.NoError(t, err)

	h.Resize(2, 2)
	h.Handle(term.Event{Ch: 'j'})

	at := term.Coordinates{X: -1}
	ed.SubscribeEditorEvents([]text.EventType{text.EventTypeScroll},
		text.FuncEventHandler(func(ctx context.Context, ev text.Event) bool {
			at = ev.Start
			return false
		}))

	h.Handle(term.Event{Ch: 'k'})
	assert.Equal(t, term.Coordinates{}, at)

	at = term.Coordinates{X: -1}
	h.Handle(term.Event{Ch: 'j'})
	assert.Equal(t, term.Coordinates{Y: 1}, at)

	at = term.Coordinates{X: -1}
	h.Handle(term.Event{Ch: 'j'})
	assert.Equal(t, term.Coordinates{X: -1}, at)
}

func TestEditorDispatchCursor(t *testing.T) {
	ed := Editor()
	buf := cell.NewBuffer()
	buf.WriteString("Matias\nGiordano\n")
	h, err := ed.Edit("zsh", buf)
	require.NoError(t, err)

	// should scroll as well, but changes in cursorAtScroll is what we are expecting
	h.Resize(2, 2)

	windowCursor := term.Coordinates{X: -1}
	scrollCursor := term.Coordinates{X: -1}
	ed.SubscribeEditorEvents([]text.EventType{text.EventTypeCursor},
		text.FuncEventHandler(func(ctx context.Context, ev text.Event) bool {
			windowCursor = ev.Start
			scrollCursor = ev.From
			assert.Equal(t, ev.ResourceName, "zsh")
			assert.Equal(t, ev.Resource, h)
			return false
		}))

	h.Handle(term.Event{Ch: 'k'})
	assert.Equal(t, term.Coordinates{X: -1}, windowCursor)
	assert.Equal(t, term.Coordinates{X: -1}, scrollCursor)

	h.Handle(term.Event{Ch: 'j'})
	assert.Equal(t, term.Coordinates{Y: 0}, windowCursor)
	assert.Equal(t, term.Coordinates{Y: 1}, scrollCursor)

	windowCursor = term.Coordinates{X: -1}
	scrollCursor = term.Coordinates{X: -1}
	h.Handle(term.Event{Ch: 'l'})
	assert.Equal(t, term.Coordinates{X: 1}, windowCursor)
	assert.Equal(t, term.Coordinates{Y: 1, X: 1}, scrollCursor)
}
