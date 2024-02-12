package term

import (
	"context"

	"github.com/ernestrc/tcell/v3"
)

type dimWriter struct {
	w Writer
}

func DimWriter(w Writer) Writer {
	return &dimWriter{w: w}
}

func (w *dimWriter) SetCell(pos Coordinates, c Cell) {
	c.Attributes.Attrs |= tcell.AttrDim
	c.Attributes.Attrs &^= tcell.AttrBold
	w.w.SetCell(pos, c)
}

func (w *dimWriter) Flush() error {
	return w.w.Flush()
}

func (w *dimWriter) Clear(attr Attributes) error {
	attr.Attrs |= tcell.AttrDim
	attr.Attrs &^= tcell.AttrBold
	return w.w.Clear(attr)
}

func (w *dimWriter) SetCursor(pos Coordinates) {
	w.w.SetCursor(pos)
}

func (w *dimWriter) Context() context.Context {
	return w.w.Context()
}
