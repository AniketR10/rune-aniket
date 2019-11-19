package cell

import (
	"io"

	"github.com/ernestrc/fractal/term"
)

// A Buffer offers a high level API to manipulate a matrix of term.Cell.
// The zero value for Buffer is ready to use.
type Buffer struct {
	cells Cells
}

// Init initializes this Buffer with the given tabspaces config and resets its contents.
func (b *Buffer) Init(tabspaces int) {
	b.cells.Init(tabspaces)
}

// Reset resets the contents of this cellbuf.
func (b *Buffer) Reset() {
	b.cells.Reset()
}

// InsertRowAt inserts a new row at given position. If pos is out of bounds,
// this method does not panic; instead, it will fill in the necessary
// rows such that the new row is the last row in the buffer.
func (b *Buffer) InsertRowAt(y int) {
	b.cells.Insert(term.Coordinates{Y: y}, "\n")
}

// NextWrite returns the position of the write cursor.
func (b *Buffer) NextWrite() term.Coordinates {
	return b.cells.NextWrite()
}

// WriteString writes the given string at the end of the buffer
func (b *Buffer) WriteString(p string) term.Coordinates {
	b.cells.Insert(b.cells.NextWrite(), p)
	return b.cells.NextWrite()
}

// WriteRune writes the given rune at the end of the buffer
func (b *Buffer) WriteRune(r rune) term.Coordinates {
	return b.WriteString(string(r))
}

// InsertAt inserts a rune in the given position and shift the cells to the right
func (b *Buffer) InsertAt(pos term.Coordinates, r rune) term.Coordinates {
	b.cells.Insert(pos, string(r))
	return b.NextWrite()
}

// DeleteRow truncates the row at term.Coordinates.Y
func (b *Buffer) DeleteRow(y int) (ok bool) {
	if ok = b.inBounds(term.Coordinates{Y: y}); !ok {
		return
	}
	from := term.Coordinates{Y: y, X: 0}
	to := term.Coordinates{Y: y, X: b.cells.Columns(y)}
	b.cells.Delete(from, to)
	return
}

func (b *Buffer) inBounds(pos term.Coordinates) (ok bool) {
	if pos.Y >= b.cells.Rows() || pos.X >= b.cells.Columns(pos.Y) {
		return
	}
	ok = true
	return
}

// TruncateRowFrom truncates the row at term.Coordinates.Y starting from term.Coordinates.X
func (b *Buffer) TruncateRowFrom(pos term.Coordinates) (ok bool) {
	if ok = b.inBounds(pos); !ok {
		return
	}
	cols := b.cells.Columns(pos.Y)
	if cols == 0 {
		ok = false
		return
	}
	b.cells.Delete(pos, term.Coordinates{Y: pos.Y, X: cols - 1})
	return
}

// TruncateFrom truncates from the given position to the end of the buffer.
func (b *Buffer) TruncateFrom(pos term.Coordinates) (ok bool) {
	if ok = b.inBounds(pos); !ok {
		return
	}
	to := term.Coordinates{X: b.cells.Columns(pos.Y), Y: b.cells.Rows() - 1}
	b.cells.Delete(pos, to)
	return
}

// ConflateRow will conflate row at index i with the next row
func (b *Buffer) ConflateRow(y int) (ok bool) {
	pos := term.Coordinates{Y: y}
	if ok = b.inBounds(pos); !ok {
		return
	}
	pos.X = b.cells.Columns(y)
	b.cells.Delete(pos, pos)
	return
}

// DeleteCell truncates the cell at the given position.
// It returns the position at which the current cell (width padding) started,
// if the width was > 1.
func (b *Buffer) DeleteCell(pos term.Coordinates) term.Coordinates {
	if ok := b.inBounds(pos); !ok {
		return term.Coordinates{}
	}

	start, _, _ := b.cells.Delete(pos, pos)
	return start
}

// Columns returns the number of cells of row at index y
func (b *Buffer) Columns(y int) (j int, ok bool) {
	pos := term.Coordinates{Y: y}
	if ok = b.inBounds(pos); !ok {
		return
	}
	j = b.cells.Columns(y)
	return
}

// Rows returns the number of rows in the buffer
func (b *Buffer) Rows() int {
	return b.cells.Rows()
}

func (b *Buffer) String() string {
	return b.cells.String()
}

// RawCells gives clients access to the underlying cell matrix.
func (b *Buffer) RawCells() [][]term.Cell {
	return b.cells.RawCells()
}

// ResetAttr resets all the attributes of the underlying cell matrix.
func (b *Buffer) ResetAttr() {
	cells := b.cells.RawCells()
	for y, r := range cells {
		for x := range r {
			cells[y][x].Fg, cells[y][x].Bg = 0, 0
		}
	}
}

// SetAttr overwrites the background and foreground attributes of cell at position.
func (b *Buffer) SetAttr(pos term.Coordinates, attr term.Attributes) (
	ok bool,
) {
	if ok = b.inBounds(pos); !ok {
		return
	}

	cells := b.cells.RawCells()
	cells[pos.Y][pos.X].Bg = attr.Bg
	cells[pos.Y][pos.X].Fg = attr.Fg
	return
}

// GetAttr gets the background and foreground attributes of cell at position.
func (b *Buffer) GetAttr(pos term.Coordinates) (
	attr term.Attributes, ok bool,
) {
	if ok = b.inBounds(pos); !ok {
		return
	}
	cells := b.cells.RawCells()
	attr = term.Attributes{Bg: cells[pos.Y][pos.X].Bg, Fg: cells[pos.Y][pos.X].Fg}
	return
}

// ReadFrom reads data from r until EOF and appends it to the buffer, growing
// the buffer as needed. The return value n is the number of bytes read. Any
// error except io.EOF encountered during the read is also returned.
func (b *Buffer) ReadFrom(r io.Reader) (n int64, err error) {
	return b.cells.ReadFrom(r)
}

// Tabspaces returns the number of tabspaces initialized.
func (b *Buffer) Tabspaces() int {
	return b.cells.Tabspaces()
}
