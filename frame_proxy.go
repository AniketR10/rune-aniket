package fractal

import "termbox"

// FrameProxy is a proxy handler that simply draws a frame around
// the underlying handler.
type FrameProxy struct {
	Frame
	handler Handler
}

// NewFrameProxy allocates storage for a new FrameProxy and initializes it.
func NewFrameProxy(handler Handler, fg, bg termbox.Attribute) (f *FrameProxy) {
	f = new(FrameProxy)
	f.Init(handler, fg, bg)
	return
}

// Init initializes this FrameProxy with the given underlying handler
// and frame attributes.
func (f *FrameProxy) Init(handler Handler, fg, bg termbox.Attribute) {
	f.handler = handler
	f.Frame.Init(handler, fg, bg)
}

// Handle delegates the event to the underlying handler.
func (f *FrameProxy) Handle(ev termbox.Event) bool {
	return f.handler.Handle(ev)
}

// GetCursor returns the underlying handler's cursor position
// with the frame offset.
func (f *FrameProxy) GetCursor() (pos Coordinates) {
	pos = f.handler.GetCursor()
	content := f.Frame.ContentPosition()
	pos.X += content.X
	pos.Y += content.Y
	return
}

// Man just delegates Man call to underlying handler.
func (f *FrameProxy) Man() Manual {
	return f.handler.Man()
}
