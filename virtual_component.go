package fractal

import (
	"fmt"
	"github.com/nsf/termbox-go"
)

// VirtualComponent wraps a component to
// provide virtual coordinates and write bound checking.
// It exposes Move which can be used to move the inner component
// in the virtual coordinate space.
type VirtualComponent struct {
	C             Component
	pos           Coordinates
	height, width int
}

type virtualWriter struct {
	writer        Writer
	offset        Coordinates
	height, width int
}

func (w *virtualWriter) Write(x, y int, ch rune, fg, bg termbox.Attribute) error {
	if x >= w.width || y >= w.height || y < 0 || x < 0 {
		panic(fmt.Sprintf("bounds check: component out of bounds: tried to write at %d,%d but width is %d and height is %d", x, y, w.width, w.height))
	}
	return w.writer.Write(w.offset.X+x, w.offset.Y+y, ch, fg, bg)
}

func (w *virtualWriter) Flush() error {
	return w.writer.Flush()
}

func (w *virtualWriter) Clear(fg, bg termbox.Attribute) error {
	return w.writer.Clear(fg, bg)
}

func (w *virtualWriter) SetCursor(pos Coordinates) {
	w.writer.SetCursor(pos)
}

// Resize resizes the underlying component and stores size
// to perform bound checking on Draw.
func (c *VirtualComponent) Resize(width, height int) {
	c.width = width
	c.height = height
	c.C.Resize(width, height)
}

// Draw uses a virtual writer to perform bound checking and
// if successful draw the inner component in the virtual coordinate space.
func (c *VirtualComponent) Draw(writer Writer) error {
	writer = &virtualWriter{
		writer: writer,
		offset: c.pos,
		height: c.height,
		width:  c.width,
	}
	return c.C.Draw(writer)
}

// Move changes the position of this virtual component
// in the virtual coordinate space.
func (c *VirtualComponent) Move(pos Coordinates) {
	c.pos = pos
}

// Width returns the width set in last Resize.
func (c *VirtualComponent) Width() int {
	return c.width
}

// Height returns the height set in last Resize.
func (c *VirtualComponent) Height() int {
	return c.height
}

// Position returns this virtual component's position
// in the virtual coordinate space.
func (c *VirtualComponent) Position() Coordinates {
	return c.pos
}
