package fractal

import "termbox"

// Container is a proxy handler that simply draws a frame around
// the underlying handler
type Container struct {
	Frame
	handler Handler
}

// NewContainer allocates storage for a new container, initializes it, and returns
// either a pointer to it or an error if there was an initialization error
func NewContainer(handler Handler, fg, bg termbox.Attribute) (c *Container) {
	c = new(Container)
	c.handler = handler
	c.Frame.Init(handler, fg, bg)
	return
}

func (c *Container) Handle(ev termbox.Event) (bool, error) {
	return c.handler.Handle(ev)
}

func (c *Container) GetCursor() Coordinates {
	return c.handler.GetCursor()
}

func (c *Container) Man() string {
	return c.handler.Man()
}
