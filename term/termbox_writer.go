//go:build !js

package term

import (
	"context"

	"github.com/unstablebuild/tcell/v3"
	"github.com/unstablebuild/tcell/v3/termbox"
)

type termboxWriter struct {
	ctx context.Context
}

func newTermboxWriter() *termboxWriter {
	ret := new(termboxWriter)
	ret.ctx = context.Background()
	return ret
}

func (w *termboxWriter) SetCell(pos Coordinates, c Cell) {
	termbox.Screen().SetContent(
		pos.X, pos.Y, c.Ch, c.Combining,
		c.Width, tcell.Style(c.Attributes),
	)
}

func (w *termboxWriter) UnionAttributes(pos Coordinates, attr Attributes) {
	termbox.Screen().UnionStyle(
		pos.X, pos.Y, tcell.Style(attr),
	)
}

func (w *termboxWriter) Flush() error {
	return termbox.Flush()
}

func (w *termboxWriter) Clear(attr Attributes) (err error) {
	termbox.Screen().Fill(' ', tcell.Style(attr))
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
