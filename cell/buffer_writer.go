package cell

import (
	"unstable.build/go-tui/term"
)

// BufferWriter satisfies term.Writer with a Buffer.
type BufferWriter struct {
	width, height int
	Cursor        term.Coordinates
	cells         [][]term.Cell
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
	w.Clear(term.Attributes{})
}

// SetCell satisfies term.Writer
func (w *BufferWriter) SetCell(pos term.Coordinates, c term.Cell) {
	if pos.X >= w.width || pos.Y >= w.height || pos.X < 0 || pos.Y < 0 {
		return
	}

	w.cells[pos.Y][pos.X].Ch = c.Ch
	w.cells[pos.Y][pos.X].Fg = c.Fg
	w.cells[pos.Y][pos.X].Bg = c.Bg
}

// Flush satisfies term.Writer
func (w *BufferWriter) Flush() error {
	return nil
}

// Clear satisfies term.Writer
func (w *BufferWriter) Clear(term.Attributes) error {
	w.cells = make([][]term.Cell, w.height)
	for i := 0; i < w.height; i++ {
		w.cells[i] = make([]term.Cell, w.width)
	}
	return nil
}

// SetCursor satisfies term.Writer
func (w *BufferWriter) SetCursor(pos term.Coordinates) {
	w.Cursor = pos
}

// ToBuffer copies the underlying cells to b.
func (w *BufferWriter) ToBuffer(b *Buffer) {
	cells := new(rawCells)
	cells.cells = w.cells
	cells.tabspaces = 1
	b.initWithCells(cells)
}

// RawCells returns the raw cells written so far to this BufferWritter.
func (w *BufferWriter) RawCells() [][]term.Cell {
	return w.cells
}
