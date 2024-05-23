//go:build !js

package term

import (
	"context"

	"github.com/unstablebuild/tcell/v3"
	"github.com/unstablebuild/tcell/v3/termbox"
)

var _ Writer = (*TermboxWriter)(nil)

// TermboxWriter implements a termbox-like API using tcell/v3.
type TermboxWriter struct {
	ctx context.Context
}

// NewTermboxWriter allocates storage for a new TermboxWriter and initializes it.
func NewTermboxWriter() *TermboxWriter {
	ret := new(TermboxWriter)
	ret.ctx = context.Background()
	return ret
}

// SetCell satisfies term.Writer.
func (w *TermboxWriter) SetCell(pos Coordinates, c Cell) {
	termbox.Screen().SetContent(
		pos.X, pos.Y, c.Ch, c.Combining,
		c.Width, tcell.Style(c.Attributes),
	)
}

// UnionAttributes satisfies term.Writer.
func (w *TermboxWriter) UnionAttributes(pos Coordinates, attr Attributes) {
	termbox.Screen().UnionStyle(
		pos.X, pos.Y, tcell.Style(attr),
	)
}

// Flush makes all the content changes made using SetCell and
// UnionAttributes visible on the display.
func (w *TermboxWriter) Flush() error {
	return termbox.Flush()
}

// Clear fills the screen with the given attributs and empty cells.
func (w *TermboxWriter) Clear(attr Attributes) (err error) {
	termbox.Screen().Fill(' ', tcell.Style(attr))
	return
}

// SetCursor displays the terminal cursor at the given location.
func (w *TermboxWriter) SetCursor(pos Coordinates) {
	termbox.SetCursor(pos.X, pos.Y)
}

// Context satisfies term.Writer.
func (w *TermboxWriter) Context() context.Context {
	return w.ctx
}

// SetContext sets the context for the next call to Context.
func (w *TermboxWriter) SetContext(ctx context.Context) {
	w.ctx = ctx
}
