package component

import (
	"fmt"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/term"
)

// VirtualComponent wraps a component to
// provide virtual coordinates and write bound checking.
// It exposes Move which can be used to move the inner component
// in the virtual coordinate space.
type VirtualComponent struct {
	C             fractal.Component
	pos           term.Coordinates
	height, width int
}

type virtualWriter struct {
	writer        fractal.Writer
	offset        term.Coordinates
	height, width int
}

func (w *virtualWriter) isOutOfBounds(pos term.Coordinates) bool {
	return pos.X >= w.width || pos.Y >= w.height || pos.Y < 0 || pos.X < 0
}

func (w *virtualWriter) assertNotOutOfBounds(pos term.Coordinates) {
	if w.isOutOfBounds(pos) {
		panic(fmt.Sprintf("bounds check: component out of bounds: tried to write at %d,%d but width is %d and height is %d",
			pos.X, pos.Y, w.width, w.height))
	}
}

func (w *virtualWriter) SetCell(pos term.Coordinates, c term.Cell) {
	w.assertNotOutOfBounds(pos)
	pos = term.Coordinates{X: w.offset.X + pos.X, Y: w.offset.Y + pos.Y}
	w.writer.SetCell(pos, c)
}

func (w *virtualWriter) SetAttr(pos term.Coordinates, attr term.Attributes) {
	w.assertNotOutOfBounds(pos)
	pos = term.Coordinates{X: w.offset.X + pos.X, Y: w.offset.Y + pos.Y}
	w.writer.SetAttr(pos, attr)
}

func (w *virtualWriter) Flush() error {
	return w.writer.Flush()
}

func (w *virtualWriter) Clear(attr term.Attributes) error {
	return w.writer.Clear(attr)
}

func (w *virtualWriter) SetCursor(pos term.Coordinates) {
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
func (c *VirtualComponent) Draw(writer fractal.Writer) {
	writer = &virtualWriter{
		writer: writer,
		offset: c.pos,
		height: c.height,
		width:  c.width,
	}
	c.C.Draw(writer)
}

// Move changes the position of this virtual component
// in the virtual coordinate space.
func (c *VirtualComponent) Move(pos term.Coordinates) {
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
func (c *VirtualComponent) Position() term.Coordinates {
	return c.pos
}
