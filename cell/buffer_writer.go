package cell

import (
	"github.com/ernestrc/go-tui/term"
)

// BufferWriter satisfies term.Writer with a Buffer.
type BufferWriter struct {
	width, height int
	Cursor        term.Coordinates
	Buffer
}

// NewBufferWriter allocates storage for a new BufferWriter and initializes it.
func NewBufferWriter(width, height int) *BufferWriter {
	ret := new(BufferWriter)
	ret.Init(width, height)
	return ret
}

// Init initializes a BufferWriter's internal structures.
func (w *BufferWriter) Init(width, height int) {
	w.width, w.height = width, height
	w.Buffer.InitWithTabspaces(1) // do not expand tabs
}

// SetCell satisfies term.Writer
func (w *BufferWriter) SetCell(pos term.Coordinates, c term.Cell) {
	if pos.X >= w.width || pos.Y >= w.height || pos.X < 0 || pos.Y < 0 {
		return
	}

	// NOTE: performance could be improved here
	_, ok := w.Buffer.Cell(pos)
	if ok {
		w.Buffer.DeleteCell(pos)
	}
	w.Buffer.InsertWithAttr(pos, c.Ch, term.Attributes{Fg: c.Fg, Bg: c.Bg})
}

// Flush satisfies term.Writer
func (w *BufferWriter) Flush() error {
	return nil
}

// Clear satisfies term.Writer
func (w *BufferWriter) Clear(term.Attributes) error {
	w.Buffer.Reset()
	return nil
}

// SetCursor satisfies term.Writer
func (w *BufferWriter) SetCursor(pos term.Coordinates) {
	w.Cursor = pos
}
