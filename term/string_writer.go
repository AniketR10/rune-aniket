package term

import (
	"bytes"
	"context"
	"fmt"
)

var _ Writer = (*StringWriter)(nil)

// StringWriter satisfies Writer by rendering the cells into a plain string.
type StringWriter struct {
	cellbuf       []Cell
	buffer        bytes.Buffer
	width, height int
	CursorCh      rune
	SetContext    context.Context
}

// NewStringWriter allocates storage for a new StringWriter and
// itnializes it.
func NewStringWriter(width, height int) (t *StringWriter) {
	t = new(StringWriter)
	t.Resize(width, height)
	t.CursorCh = '▐'
	t.SetContext = context.Background()
	return
}

// Context returns context.Background
func (w *StringWriter) Context() context.Context {
	return w.SetContext
}

// Resize satisfies Writer.
func (w *StringWriter) Resize(width, height int) {
	w.width, w.height = width, height
	w.cellbuf = make([]Cell, width*height)
}

func outOfBounds(height, width int, pos Coordinates) bool {
	return pos.X >= width || pos.Y >= height || pos.X < 0 || pos.Y < 0
}

// SetCell satisfies Writer.
func (w *StringWriter) SetCell(pos Coordinates, cell Cell) {
	if outOfBounds(w.height, w.width, pos) {
		panic(fmt.Sprintf("SetCell(x=%d;y=%d): out of bounds: width=%d;height=%d",
			pos.X, pos.Y, w.width, w.height))
	}
	idx := pos.Y*w.width + pos.X
	w.cellbuf[idx] = cell
}

// Flush satisfies Writer.
func (w *StringWriter) Flush() (err error) {
	for i, c := range w.cellbuf {
		if i != 0 && i%w.width == 0 {
			w.buffer.WriteRune('\n')
		}
		ch := c.Ch
		switch ch {
		case '\t', '\n', 0:
			ch = ' '
		}
		w.buffer.WriteRune(ch)
	}
	return
}

// Cells returns the internal cell slice.
func (w *StringWriter) Cells() []Cell {
	return w.cellbuf
}

// Clear satisfies Writer. Note that attr are ignored as they
// can't be represented in a string.
func (w *StringWriter) Clear(attr Attributes) (err error) {
	w.cellbuf = make([]Cell, w.width*w.height)
	w.buffer.Reset()
	return
}

func (w *StringWriter) String() string {
	return w.buffer.String()
}

// SetCursor satisfies Writer by substituting the rune
// at pos for a pre-defined cursor-like rune.
func (w *StringWriter) SetCursor(pos Coordinates) {
	i := pos.X + pos.Y*w.width
	if i < len(w.cellbuf) {
		w.cellbuf[i].Ch = w.CursorCh
	}
}
