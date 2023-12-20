//go:build !js

package term

import (
	"context"

	"github.com/ernestrc/tcell/v2/termbox"
)

// ContextWriter adds SetContext to a Writer.
type ContextWriter interface {
	Writer
	SetContext(context.Context)
}

type termboxWriter struct {
	ctx context.Context
}

func newTermboxWriter() *termboxWriter {
	ret := new(termboxWriter)
	ret.ctx = context.Background()
	return ret
}

func (w *termboxWriter) SetCell(pos Coordinates, c Cell) {
	termbox.SetCell(pos.X, pos.Y,
		c.Ch, termbox.Attribute(c.Fg), termbox.Attribute(c.Bg))
	return
}

func (w *termboxWriter) Flush() error {
	return termbox.Flush()
}

func (w *termboxWriter) Clear(attr Attributes) (err error) {
	termbox.Clear(termbox.Attribute(attr.Fg), termbox.Attribute(attr.Bg))
	return
}

func (w *termboxWriter) SetCursor(pos Coordinates) {
	termbox.SetCursor(pos.X, pos.Y)
}

func (w *termboxWriter) Context() context.Context {
	return w.ctx
}

func (w *termboxWriter) SetContext(ctx context.Context) {
	w.ctx = ctx
}
