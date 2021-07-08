package vi

import (
	"testing"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEditorDispatchFocus(t *testing.T) {
	ed := Editor()
	content := "Clement"
	name := "Jolie"

	var h editor.Handler
	ed.SubscribeEditorEvents(editor.EventTypeOpen, editor.FuncEventHandler(func(ev editor.Event) bool {
		assert.Equal(t, editor.EventTypeOpen, ev.Type)
		assert.Equal(t, content, ev.Content)
		assert.Equal(t, name, ev.ResourceName)
		h = ev.Resource
		return false
	}))

	var focusCalled int
	ed.SubscribeEditorEvents(editor.EventTypeFocus, editor.FuncEventHandler(func(ev editor.Event) bool {
		focusCalled++
		assert.Equal(t, editor.EventTypeFocus, ev.Type)
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
	buf.WriteString("Daworg\nSurinach\n")
	h, err := ed.Edit("oh my...", buf)
	require.NoError(t, err)

	h.Resize(2, 2)
	require.True(t, h.(*Vi).less.Scroll.SeekDown())

	at := term.Coordinates{X: -1}
	ed.SubscribeEditorEvents(editor.EventTypeScroll, editor.FuncEventHandler(func(ev editor.Event) bool {
		at = ev.Start
		return false
	}))

	require.True(t, h.(*Vi).less.Scroll.SeekUp())
	assert.Equal(t, term.Coordinates{}, at)

	at = term.Coordinates{X: -1}
	require.True(t, h.(*Vi).less.Scroll.SeekDown())
	assert.Equal(t, term.Coordinates{Y: 1}, at)

	at = term.Coordinates{X: -1}
	require.False(t, h.(*Vi).less.Scroll.SeekDown())
	assert.Equal(t, term.Coordinates{X: -1}, at)
}
