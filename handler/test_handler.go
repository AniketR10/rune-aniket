package handler

import (
	"strings"
	"testing"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/component"
	"github.com/ernestrc/fractal/term"
)

// TestHandler is a handler used to test composite handlers. Each event
// processed by this handler increments the Ch rune to the next rune.
type TestHandler struct {
	component.TestComponent
	fractal.Manual
	Exit bool
}

// NewTestHandler will allocate storage for a new handler and initialize it
func NewTestHandler() (t *TestHandler) {
	t = new(TestHandler)
	t.Ch = 'A'
	return
}

// Handle the next Event
func (t *TestHandler) Handle(term.Event) bool {
	// signal that we handled the event
	t.Ch++
	return t.Exit
}

// Cursor returns always a hidden cursor
func (t *TestHandler) Cursor() (term.Coordinates, bool) {
	return term.Coordinates{X: -1, Y: -1}, false
}

// Man for this handler is empty
func (t *TestHandler) Man() fractal.Manual {
	return t.Manual
}

type handlerTestCase struct {
	event    term.Event
	expected string
}

func testHandlerWorkflow(
	t *testing.T,
	handler fractal.Handler,
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
		if expected != w.String() {
			t.Errorf("expected %q found %q", expected, w.String())
		}
	}
}
