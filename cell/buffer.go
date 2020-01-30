package cell

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/ernestrc/fractal/term"
	log "github.com/sirupsen/logrus"
)

// A Buffer offers a high level API to manipulate a matrix of term.Cell.
type Buffer struct {
	tabspaces  int
	readerFrom io.ReaderFrom
	reader     Reader
	writer     Writer
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

	b.tabspaces = tabspaces
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

// WithReader sets this Buffer's Reader to r.
func (b *Buffer) WithReader(r Reader) *Buffer {
	b.reader = r
	return b
}

// WithWriter sets this Buffer's Writer to w.
func (b *Buffer) WithWriter(w Writer) *Buffer {
	b.writer = w
	return b
}

// Reader returns this Buffer's underlying cell.Reader.
func (b *Buffer) Reader() Reader {
	return b.reader
}

// Writer returns this Buffer's underlying cell.Writer.
func (b *Buffer) Writer() Writer {
	return b.writer
}

// WithLogger adds a cell logger which intercepts and logs all
// the calls to the underlying Writer/Reader.
func (b *Buffer) WithLogger(logger *log.Logger) *Buffer {
	cellLogger := newLogger(b.Reader(), b.Writer(), logger)
	return b.WithReader(cellLogger).WithWriter(cellLogger)
}

// InsertRowAt inserts a new row at given position. If pos is out of bounds,
// this method does not panic; instead, it will fill in the necessary
// rows such that the new row is the last row in the buffer.
func (b *Buffer) InsertRowAt(y int) {
	b.writer.Insert(term.Coordinates{Y: y}, "\n")
}

// Insert inserts a rune in the given position and shift the cells to the right
func (b *Buffer) Insert(pos term.Coordinates, r rune) (next term.Coordinates) {
	_, next = b.writer.Insert(pos, string(r))

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
	if ok = y < b.reader.Rows(); !ok {
		return
	}
	from := term.Coordinates{Y: y, X: 0}
	to := term.Coordinates{Y: y, X: b.reader.Columns(y)}
	b.writer.Delete(from, to)
	return
}

// used by methods that need to validate cell access.
// inBounds just checks that it's a valid coordinate for the underlying
// reader/writer, which for instance could be x = len(row), which
// does not contain a cell.
func (b *Buffer) inStrictBounds(pos term.Coordinates) (ok bool) {
	if pos.Y >= b.reader.Rows() || pos.X >= b.reader.Columns(pos.Y) {
		return
	}
	ok = true
	return
}

func (b *Buffer) inBounds(pos term.Coordinates) (ok bool) {
	if pos.Y >= b.reader.Rows() || pos.X > b.reader.Columns(pos.Y) {
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
	cols := b.reader.Columns(pos.Y)
	if cols == 0 {
		ok = false
		return
	}
	b.writer.Delete(pos, term.Coordinates{Y: pos.Y, X: cols - 1})
	return
}

// TruncateFrom truncates from the given position to the end of the buffer.
func (b *Buffer) TruncateFrom(pos term.Coordinates) (ok bool) {
	if ok = b.inBounds(pos); !ok {
		return
	}
	y := b.reader.Rows() - 1
	to := term.Coordinates{X: b.reader.Columns(y), Y: y}
	b.writer.Delete(pos, to)
	return
}

// ConflateRow will conflate row at index i with the next row
func (b *Buffer) ConflateRow(y int) (ok bool) {
	pos := term.Coordinates{Y: y}
	if ok = b.inBounds(pos); !ok {
		return
	}
	pos.X = b.reader.Columns(y)
	b.writer.Delete(pos, pos)
	return
}

// DeleteCell truncates the cell at the given position.
// It returns the position at which the current cell (width padding) started,
// if the width was > 1.
func (b *Buffer) DeleteCell(pos term.Coordinates) (term.Coordinates, bool) {
	if ok := b.inBounds(pos); !ok {
		return term.Coordinates{}, false
	}

	start, _, _ := b.writer.Delete(pos, pos)
	return start, true
}

// ResetAttr resets all the attributes of the underlying cell matrix.
func (b *Buffer) ResetAttr() {
	cells := b.reader.RawCells()
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
	if ok = b.inStrictBounds(pos); !ok {
		return
	}

	cells := b.reader.RawCells()
	cells[pos.Y][pos.X].Bg = attr.Bg
	cells[pos.Y][pos.X].Fg = attr.Fg
	return
}

// GetAttr gets the background and foreground attributes of cell at position.
func (b *Buffer) GetAttr(pos term.Coordinates) (
	attr term.Attributes, ok bool,
) {
	if ok = b.inStrictBounds(pos); !ok {
		return
	}
	cells := b.reader.RawCells()
	attr = term.Attributes{Bg: cells[pos.Y][pos.X].Bg, Fg: cells[pos.Y][pos.X].Fg}
	return
}

// Rows returns the number of rows in the Buffer.
func (b *Buffer) Rows() int {
	return b.reader.Rows()
}

// Columns returns the number of cells of row at index y.
func (b *Buffer) Columns(y int) int {
	return b.reader.Columns(y)
}

// Cell returns the cell and true or a zero-valued cell and false if there is no
// cell at position.
func (b *Buffer) Cell(pos term.Coordinates) (term.Cell, bool) {
	return b.reader.Cell(pos)
}

// RawCells gives clients access to the underlying cell matrix.
func (b *Buffer) RawCells() [][]term.Cell {
	return b.reader.RawCells()
}

// InsertString inserts string in the given position and shifts the remaining cells.
// insert never fails: if at is out-of-bounds, this method fills in the rows
// and/or columns of cells with blank spaces.
// It returns the start of the insert 'from', including the filled-in blank spaces
// and where the next logical Insert should go 'until'.
func (b *Buffer) InsertString(at term.Coordinates, str string) (from, until term.Coordinates) {
	from, until = b.writer.Insert(at, str)

	if str == "" {
		return
	}
	runes := []rune(str)
	if runes[len(runes)-1] == '\n' {
		until.Y++
		until.X = 0
		return
	}
	until.X++
	return
}

// Delete removes cells in left-inclusive, right-inclusive range
// and returns the corresponding string representation of the cells removed,
// along with the true start and end of the range, in case some cells groups
// (cell with padding) were deleted.
// Note that if to.X == b.Columns(to.Y), the newline at the end of the row is deleted,
// and so row is conflated with next row.
func (b *Buffer) Delete(from, to term.Coordinates) (start, end term.Coordinates, str string) {
	if !b.inBounds(from) || !b.inBounds(to) {
		panic(fmt.Sprintf("out of bounds: from=%+v, to=%+v", from, to))
	}
	return b.writer.Delete(from, to)
}

// Reset resets the contents of this Buffer.
func (b *Buffer) Reset() {
	b.InitWithTabspaces(b.tabspaces)
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
func (b *Buffer) Undo() (bool, term.Coordinates) {
	return b.undoer.undo()
}

// Redo reverses the previously reversed update to the Buffer.
func (b *Buffer) Redo() (bool, term.Coordinates) {
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
