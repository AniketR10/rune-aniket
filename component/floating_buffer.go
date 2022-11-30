package component

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/cell"
)

// FloatingBuffer wraps a tui.Component and uses the contents of buffer
// do determine the best dimensions for the given component.
func FloatingBuffer(c tui.Component, buffer *cell.Buffer) Floating {
	return floatingBuffer{Component: c, buffer: buffer}
}

type floatingBuffer struct {
	tui.Component
	buffer *cell.Buffer
}

func (f floatingBuffer) Dimensions() (width, height int) {
	width = f.buffer.MaxColumns()
	height = f.buffer.Rows()
	return
}
