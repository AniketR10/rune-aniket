package test

import (
	"strings"
	"testing"
	"time"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HandlerSequenceTestCase represents an input sequence and
// the result expected draw string representation.
type HandlerSequenceTestCase struct {
	InputSequence string
	Expected      string
}

// HandlerTestCase represents an event and the result
// expected draw string representation.
type HandlerTestCase struct {
	Event    term.Event
	Expected string
}

// TestHandlerSequence is a helper function that drives
// a set of HandlerSequenceTestCase and its results.
//
// Certain key events are encoded in characters. For instance, a '>' character
// signals term.KeyEnter and '<' character signals term.KeyEsc.
func TestHandlerSequence(
	t *testing.T, handler tui.Handler, width, height int,
	cases []HandlerSequenceTestCase,
) {
	writer := term.NewStringWriter(width, height)
	handler.Resize(width, height)

	for _, tcase := range cases {
		err := writer.Clear(term.Attributes{Fg: 0, Bg: 0})
		require.NoError(t, err)

		var shouldSleep bool
		for _, r := range tcase.InputSequence {
			switch r {
			case ':':
				handler.Handle(term.Event{Key: term.KeyCtrlBackslash, Type: term.EventKey})
			case '_':
				shouldSleep = true
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

		// this is a hack for async handlers
		if shouldSleep {
			// wait until all events have been dispatched
			time.Sleep(100 * time.Millisecond)
			// force a draw and wait for the draw response to arrive
			handler.Draw(term.NewStringWriter(width, height))
			time.Sleep(100 * time.Millisecond)
		}
		handler.Draw(writer)

		cursor, ok := handler.Cursor()
		if ok {
			writer.SetCursor(cursor)
		}

		err = writer.Flush()
		require.NoError(t, err)

		out := writer.String()
		assert.Equal(t, tcase.Expected, out)
	}
}

// TestHandler is a helper function that tests a handler against
// a sequence of HandlerTestCase.
func TestHandler(
	t *testing.T, handler tui.Handler,
	cases []HandlerTestCase, w *term.StringWriter,
) {
	var err error

	for _, tcase := range cases {
		if err = w.Clear(term.Attributes{}); err != nil {
			t.Fatal(err)
		}

		handler.Handle(tcase.Event)

		handler.Draw(w)

		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}

		// for readability, we expected strings are written starting with \n
		expected := strings.TrimLeft(tcase.Expected, "\n")
		assert.Equal(t, expected, w.String())
	}
}
