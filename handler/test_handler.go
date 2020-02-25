package handler

import (
	"strings"
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHandler is a handler used to test composite handlers. Each event
// processed by this handler increments the Ch rune to the next rune.
type TestHandler struct {
	component.TestComponent
	tui.Manual
	Exit bool
}

// NewTestHandler will allocate storage for a new handler and initialize it
func NewTestHandler() (t *TestHandler) {
	t = new(TestHandler)
	t.Ch = 'A'
	return
}

// Handle the next Event
func (t *TestHandler) Handle(term.Event) (bool, bool) {
	// signal that we handled the event
	t.Ch++
	return t.Exit, true
}

// Cursor returns always a hidden cursor
func (t *TestHandler) Cursor() (term.Coordinates, bool) {
	return term.Coordinates{X: -1, Y: -1}, false
}

// Man for this handler is empty
func (t *TestHandler) Man() tui.Manual {
	return t.Manual
}

type handlerTestCase struct {
	event    term.Event
	expected string
}

func testHandlerWorkflow(
	t *testing.T,
	handler tui.Handler,
	cases []handlerTestCase,
	w *term.StringWriter,
) {
	var err error

	for _, tcase := range cases {
		if err = w.Clear(term.Attributes{}); err != nil {
			t.Fatal(err)
		}

		handler.Handle(tcase.event)

		handler.Draw(w)

		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}

		// for readability, we expected strings are written starting with \n
		expected := strings.TrimLeft(tcase.expected, "\n")
		assert.Equal(t, expected, w.String())
	}
}

// TestInputSequence represents an input sequence and
// the result expected draw string representation.
type TestInputSequence struct {
	InputSequence string
	DrawOutput    string
}

// BatchTestInputSequence is a helper function that drives
// a set of TestInputSequence and its results.
//
// Certain key events are encoded in characters. For instance, a '>' character
// signals term.KeyEnter and '<' character signals term.KeyEsc.
func BatchTestInputSequence(
	t *testing.T, handler tui.Handler, width, height int, cases []TestInputSequence,
) {
	writer := term.NewStringWriter(width, height)
	handler.Resize(width, height)

	for _, tcase := range cases {
		err := writer.Clear(term.Attributes{Fg: 0, Bg: 0})
		require.NoError(t, err)

		for _, r := range tcase.InputSequence {
			switch r {
			case ' ':
				handler.Handle(term.Event{Key: term.KeySpace, Type: term.EventKey})
			case '^':
				handler.Handle(term.Event{Key: term.KeyBackspace, Type: term.EventKey})
			case '#':
				handler.Handle(term.Event{Key: term.KeyCtrlH, Type: term.EventKey})
			case '$':
				handler.Handle(term.Event{Key: term.KeyCtrlL, Type: term.EventKey})
			case '>':
				handler.Handle(term.Event{Key: term.KeyEnter, Type: term.EventKey})
			case '<':
				handler.Handle(term.Event{Key: term.KeyEsc, Type: term.EventKey})
			default:
				handler.Handle(term.Event{Ch: r, Type: term.EventKey})
			}
		}

		handler.Draw(writer)

		cursor, ok := handler.Cursor()
		if ok {
			writer.SetCursor(cursor)
		}

		err = writer.Flush()
		require.NoError(t, err)

		out := writer.String()
		if client, ok := handler.(*Client); ok {
			assert.Equal(t, tcase.DrawOutput, out, client.errors)
		} else {
			assert.Equal(t, tcase.DrawOutput, out)
		}
	}
}
