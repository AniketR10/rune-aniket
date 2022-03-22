//go:build !js
// +build !js

package term

import (
	"github.com/nsf/termbox-go"
	// TODO should use
	// https://github.com/gdamore/tcell/blob/master/termbox/compat.go
)

type termboxWriter struct{}

func (w termboxWriter) SetCell(pos Coordinates, c Cell) {
	termbox.SetCell(pos.X, pos.Y,
		c.Ch, termbox.Attribute(c.Fg), termbox.Attribute(c.Bg))
	return
}

func (w termboxWriter) Flush() error {
	return termbox.Flush()
}

func (w termboxWriter) Clear(attr Attributes) (err error) {
	err = termbox.Clear(termbox.Attribute(attr.Fg), termbox.Attribute(attr.Bg))
	return
}

func (w termboxWriter) SetCursor(pos Coordinates) {
	termbox.SetCursor(pos.X, pos.Y)
}
