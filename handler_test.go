package fractal

import (
	"github.com/nsf/termbox-go"
)

// TestHandler is a handler used to test composite handlers. Each event
// processed by this handler increments the Ch rune to the next rune.
type TestHandler struct {
	TestComponent
	Manual
	Exit bool
}

// NewTestHandler will allocate storage for a new handler and initialize it
func NewTestHandler() (t *TestHandler) {
	t = new(TestHandler)
	t.Ch = 'A'
	return
}

// Handle the next Event
func (t *TestHandler) Handle(termbox.Event) bool {
	// signal that we handled the event
	t.Ch++
	return t.Exit
}

// GetCursor returns always a hidden cursor
func (t *TestHandler) GetCursor() Coordinates {
	return Coordinates{X: -1, Y: -1}
}

// Man for this handler is empty
func (t *TestHandler) Man() Manual {
	return t.Manual
}
