package handler

import (
	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/component"
	"github.com/ernestrc/fractal/term"
)

// Frame is a proxy handler that simply draws a frame around
// the underlying handler.
type Frame struct {
	component.Frame
	handler fractal.Handler
}

// NewFrame allocates storage for a new Frame and initializes it.
func NewFrame(handler fractal.Handler) (f *Frame) {
	f = new(Frame)
	f.Init(handler)
	return
}

// Init initializes this Frame with the given underlying handler
// and frame attributes.
func (f *Frame) Init(handler fractal.Handler) {
	f.handler = handler
	f.Frame.Init(handler)
}

// Handle delegates the event to the underlying handler.
func (f *Frame) Handle(ev term.Event) bool {
	return f.handler.Handle(ev)
}

// Cursor returns the underlying handler's cursor position
// with the frame offset.
func (f *Frame) Cursor() (pos term.Coordinates, show bool) {
	pos, show = f.handler.Cursor()
	content := f.Frame.ContentPosition()
	pos.X += content.X
	pos.Y += content.Y
	return
}

// Man just delegates Man call to underlying handler.
func (f *Frame) Man() fractal.Manual {
	return f.handler.Man()
}
