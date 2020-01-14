package cell

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/ernestrc/fractal/term"
)

// A Buffer offers a high level API to manipulate a matrix of term.Cell.
type Buffer struct {
	readerFrom io.ReaderFrom
	reader     reader
	writer     writer
	undoer     *undoer
	selector   selector
}

// NewBuffer allocates storage for a new Buffer and initializes it.
func NewBuffer() (b *Buffer) {
	b = new(Buffer)
	b.Init()
	return b
}

// InitWithTabspaces initializes this Buffer with
func (b *Buffer) InitWithTabspaces(tabspaces int) {
	cells := new(rawCells)
	cells.init(tabspaces)

	b.readerFrom = cells
	b.reader = newUnixFileBuffer(cells)

	b.undoer = newUndoer(cells)
	b.writer = b.undoer

	b.selector.reader = b.reader

}

// Init initializes this Buffer with the default configuration.
func (b *Buffer) Init() {
	b.InitWithTabspaces(defTabSpaces)
}

// WithLogger adds a cell logger this Buffer which
// intercepts all the calls and logs them to out.
func (b *Buffer) WithLogger(out io.Writer) *Buffer {
	b.reader, b.writer = newLogger(b.reader, b.writer, out)
	return b
}

// InsertRowAt inserts a new row at given position. If pos is out of bounds,
// this method does not panic; instead, it will fill in the necessary
// rows such that the new row is the last row in the buffer.
func (b *Buffer) InsertRowAt(y int) {
	b.writer.insert(term.Coordinates{Y: y}, "\n")
}

// InsertAt inserts a rune in the given position and shift the cells to the right
func (b *Buffer) InsertAt(pos term.Coordinates, r rune) (next term.Coordinates) {
	_, next = b.writer.insert(pos, string(r))

	if r == '\n' {
		next.Y++
		next.X = 0
		return
	}

	next.X++

	return
}

// DeleteRow truncates the row at term.Coordinates.Y
func (b *Buffer) DeleteRow(y int) (ok bool) {
	if ok = y < b.reader.rows(); !ok {
		return
	}
	from := term.Coordinates{Y: y, X: 0}
	to := term.Coordinates{Y: y, X: b.reader.columns(y)}
	b.writer.delete(from, to)
	return
}

func (b *Buffer) inBounds(pos term.Coordinates) (ok bool) {
	if pos.Y >= b.reader.rows() || pos.X >= b.reader.columns(pos.Y) {
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
	cols := b.reader.columns(pos.Y)
	if cols == 0 {
		ok = false
		return
	}
	b.writer.delete(pos, term.Coordinates{Y: pos.Y, X: cols - 1})
	return
}

// TruncateFrom truncates from the given position to the end of the buffer.
func (b *Buffer) TruncateFrom(pos term.Coordinates) (ok bool) {
	if ok = b.inBounds(pos); !ok {
		return
	}
	y := b.reader.rows() - 1
	to := term.Coordinates{X: b.reader.columns(y), Y: y}
	b.writer.delete(pos, to)
	return
}

// ConflateRow will conflate row at index i with the next row
func (b *Buffer) ConflateRow(y int) (ok bool) {
	pos := term.Coordinates{Y: y}
	if ok = b.inBounds(pos); !ok {
		return
	}
	pos.X = b.reader.columns(y)
	b.writer.delete(pos, pos)
	return
}

// DeleteCell truncates the cell at the given position.
// It returns the position at which the current cell (width padding) started,
// if the width was > 1.
func (b *Buffer) DeleteCell(pos term.Coordinates) (term.Coordinates, bool) {
	if ok := b.inBounds(pos); !ok {
		return term.Coordinates{}, false
	}

	start, _, _ := b.writer.delete(pos, pos)
	return start, true
}

// ResetAttr resets all the attributes of the underlying cell matrix.
func (b *Buffer) ResetAttr() {
	cells := b.reader.rawCells()
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

	cells := b.reader.rawCells()
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
	cells := b.reader.rawCells()
	attr = term.Attributes{Bg: cells[pos.Y][pos.X].Bg, Fg: cells[pos.Y][pos.X].Fg}
	return
}

// Rows returns the number of rows in the Buffer.
func (b *Buffer) Rows() int {
	return b.reader.rows()
}

// Columns returns the number of cells of row at index y.
func (b *Buffer) Columns(y int) int {
	return b.reader.columns(y)
}

// Cell returns the cell and true or a zero-valued cell and false if there is no
// cell at position. If attempting to get a tab padding, the position of the
// tab is returned.
func (b *Buffer) Cell(pos term.Coordinates) (term.Coordinates, term.Cell) {
	return b.reader.cell(pos)
}

// RawCells gives clients access to the underlying cell matrix.
func (b *Buffer) RawCells() [][]term.Cell {
	return b.reader.rawCells()
}

// Insert inserts string in the given position and shifts the remaining cells.
// insert never fails: if at is out-of-bounds, this method fills in the rows
// and/or columns of cells.
func (b *Buffer) Insert(at term.Coordinates, str string) (from, to term.Coordinates) {
	return b.writer.insert(at, str)
}

// Delete removes cells in left-inclusive, right-inclusive range
// and returns the corresponding string representation of the cells removed,
// along with the true start and end of the range, in case some cells groups
// (cell with padding) were deleted.
func (b *Buffer) Delete(from, to term.Coordinates) (start, end term.Coordinates, str string) {
	if !b.inBounds(from) || !b.inBounds(to) {
		panic(fmt.Sprintf("out of bounds: from=%+v, to=%+v", from, to))
	}
	return b.writer.delete(from, to)
}

// Reset resets the contents of this Buffer.
func (b *Buffer) Reset() {
	b.writer.reset()
	b.undoer.reset()
}

// ReadFrom reads data from r until EOF and appends it to the buffer, growing
// the buffer as needed. The return value n is the number of bytes read. Any
// error except io.EOF encountered during the read is also returned.
func (b *Buffer) ReadFrom(r io.Reader) (int64, error) {
	return b.readerFrom.ReadFrom(r)
}

// io.Writer
func (b *Buffer) Write(p []byte) (int, error) {
	n, err := b.readerFrom.ReadFrom(bytes.NewReader(p))
	return int(n), err
}

// WriteString writes the given string at the end of the buffer
func (b *Buffer) WriteString(p string) {
	_, err := b.readerFrom.ReadFrom(strings.NewReader(p))
	if err != nil {
		// strings.Reader never errors out
		panic(err)
	}
}

// Undo reverses the last update to the Buffer.
// Redo can be used to reverse Undo.
func (b *Buffer) Undo() bool {
	return b.undoer.undo()
}

// Redo reverses the previously reversed update to the Buffer.
func (b *Buffer) Redo() bool {
	return b.undoer.redo()
}

// Select returns the cells inside the given coordinates or nil if coordinates
// are out of bounds.
func (b *Buffer) Select(from term.Coordinates, to term.Coordinates) [][]term.Cell {
	return b.selector.selectCells(from, to)
}

// SelectLine returns the lines inside the given coordinates or nil if
// coordinates are out of bounds.
func (b *Buffer) SelectLine(from term.Coordinates, to term.Coordinates) [][]term.Cell {
	return b.selector.selectLine(from, to)
}

// SelectBlock returns the block of cells inside the given coordinates or nil if
// coordinates are out of bounds.
func (b *Buffer) SelectBlock(from term.Coordinates, to term.Coordinates) [][]term.Cell {
	return b.selector.selectBlock(from, to)
}

func (b *Buffer) String() string {
	return b.reader.String()
}
