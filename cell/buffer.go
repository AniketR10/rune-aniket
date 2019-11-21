package cell

import (
	"github.com/ernestrc/fractal/term"
)

// A Buffer offers a high level API to manipulate cell.ReadWriter.
type Buffer struct {
	Writer
	Reader
}

// NewBuffer allocates storage for a new Buffer and initializes it.
func NewBuffer() (b *Buffer) {
	b = new(Buffer)
	b.Init()
	return b
}

// Init initializes this Buffer with the given instance of ReadWriter.
func (b *Buffer) Init() {
	cells := new(RawCells)
	b.Writer = cells
	b.Reader = cells
}

// SetReadWriter sets this buffer writer and reader to rw.
func (b *Buffer) SetReadWriter(rw ReadWriter) {
	b.Writer = rw
	b.Reader = rw
}

// InsertRowAt inserts a new row at given position. If pos is out of bounds,
// this method does not panic; instead, it will fill in the necessary
// rows such that the new row is the last row in the buffer.
func (b *Buffer) InsertRowAt(y int) {
	b.Writer.Insert(term.Coordinates{Y: y}, "\n")
}

// NextWrite returns the position of the write cursor.
func (b *Buffer) NextWrite() term.Coordinates {
	return b.Writer.NextWrite()
}

// WriteString writes the given string at the end of the buffer
func (b *Buffer) WriteString(p string) term.Coordinates {
	b.Writer.Insert(b.Writer.NextWrite(), p)
	return b.Writer.NextWrite()
}

// WriteRune writes the given rune at the end of the buffer
func (b *Buffer) WriteRune(r rune) term.Coordinates {
	return b.WriteString(string(r))
}

// InsertAt inserts a rune in the given position and shift the cells to the right
func (b *Buffer) InsertAt(pos term.Coordinates, r rune) term.Coordinates {
	b.Writer.Insert(pos, string(r))
	return b.Writer.NextWrite()
}

// DeleteRow truncates the row at term.Coordinates.Y
func (b *Buffer) DeleteRow(y int) (ok bool) {
	if ok = b.inBounds(term.Coordinates{Y: y}); !ok {
		return
	}
	from := term.Coordinates{Y: y, X: 0}
	to := term.Coordinates{Y: y, X: b.Reader.Columns(y)}
	b.Writer.Delete(from, to)
	return
}

func (b *Buffer) inBounds(pos term.Coordinates) (ok bool) {
	if pos.Y >= b.Reader.Rows() || pos.X >= b.Reader.Columns(pos.Y) {
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
	cols := b.Reader.Columns(pos.Y)
	if cols == 0 {
		ok = false
		return
	}
	b.Writer.Delete(pos, term.Coordinates{Y: pos.Y, X: cols - 1})
	return
}

// TruncateFrom truncates from the given position to the end of the buffer.
func (b *Buffer) TruncateFrom(pos term.Coordinates) (ok bool) {
	if ok = b.inBounds(pos); !ok {
		return
	}
	y := b.Reader.Rows() - 1
	to := term.Coordinates{X: b.Reader.Columns(y), Y: y}
	b.Writer.Delete(pos, to)
	return
}

// ConflateRow will conflate row at index i with the next row
func (b *Buffer) ConflateRow(y int) (ok bool) {
	pos := term.Coordinates{Y: y}
	if ok = b.inBounds(pos); !ok {
		return
	}
	pos.X = b.Reader.Columns(y)
	b.Writer.Delete(pos, pos)
	return
}

// DeleteCell truncates the cell at the given position.
// It returns the position at which the current cell (width padding) started,
// if the width was > 1.
func (b *Buffer) DeleteCell(pos term.Coordinates) term.Coordinates {
	if ok := b.inBounds(pos); !ok {
		return term.Coordinates{}
	}

	start, _, _ := b.Writer.Delete(pos, pos)
	return start
}

/* NOTE: the following methods should be removed and employ an attributes view */

// ResetAttr resets all the attributes of the underlying cell matrix.
func (b *Buffer) ResetAttr() {
	cells := b.Reader.RawCells()
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

	cells := b.Reader.RawCells()
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
	cells := b.Reader.RawCells()
	attr = term.Attributes{Bg: cells[pos.Y][pos.X].Bg, Fg: cells[pos.Y][pos.X].Fg}
	return
}
