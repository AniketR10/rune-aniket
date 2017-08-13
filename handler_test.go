package fractal

import (
	"termbox"
)

// TestHandler is a handler used to test composite handlers. Each event
// processed by this handler increments the Ch rune to the next rune.
type TestHandler struct {
	fill TestComponent
	Frame
}

// NewTestHandler will allocate storage for a new handler and initialize it
func NewTestHandler() *TestHandler {
	t := new(TestHandler)
	t.fill.Ch = 'A'
	if err := t.SetContent(&t.fill); err != nil {
		panic(err)
	}
	return t
}

// Handle the next Event
func (t *TestHandler) Handle(termbox.Event) (bool, error) {
	// signal that we handled the event
	t.fill.Ch++
	return false, nil
}

// GetCursor returns always a hidden cursor
func (t *TestHandler) GetCursor() Coordinates {
	return Coordinates{X: -1, Y: -1}
}

// Man for this handler is empty
func (t *TestHandler) Man() string {
	return ""
}
