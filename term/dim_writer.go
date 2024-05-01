package term

import (
	"context"

	"github.com/unstablebuild/tcell/v3"
)

type dimWriter struct {
	w Writer
}

// DimWriter returns a Writer that sets tcell.AttrDim
// and removes tcell.AttrBold to all cells by default.
func DimWriter(w Writer) Writer {
	return &dimWriter{w: w}
}

func (w *dimWriter) SetCell(pos Coordinates, c Cell) {
	c.Attributes.Attrs |= tcell.AttrDim
	c.Attributes.Attrs &^= tcell.AttrBold
	w.w.SetCell(pos, c)
}

func (w *dimWriter) UnionAttributes(pos Coordinates, attr Attributes) {
	attr.Attrs |= tcell.AttrDim
	attr.Attrs &^= tcell.AttrBold
	w.w.UnionAttributes(pos, attr)
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
