package cell

import (
	"io"
	"strings"

	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

// A Buffer offers a high level API to manipulate a matrix of term.Cell.
type Buffer struct {
	cells    *rawCells
	undoer   *undoer
	selector selector
	rootPub  *syncPublisher
	usagePub *syncPublisher
	safew    Editor

	// effective View and Editor
	view   View
	editor Editor
}

type safeEditor struct {
	editor Editor
	view   View
}

func fromToInBounds(cells View, from, to term.Coordinates) (
	newFrom, newTo term.Coordinates, ok bool,
) {
	rows := cells.Rows()
	if rows == 0 || from.Y >= rows || (from.Y == rows-1 && from.X > cells.Columns(from.Y)) {
		return
	}

	if cols := cells.Columns(from.Y); from.X > cols {
		from.X = cols
	}

	if to.Y >= rows {
		to.Y = rows
		to.X = 0
	} else if cols := cells.Columns(to.Y); to.X > cols {
		to.X = cols
	}

	return from, to, true
}

func (s safeEditor) Edit(start, end term.Coordinates, str string) (
	from, to term.Coordinates, old string,
) {
	// only check in case of delete range
	if start != end {
		var ok bool
		start, end, ok = fromToInBounds(s.view, start, end)
		if !ok {
			return
		}
	}
	return s.editor.Edit(start, end, str)
}

// NewBuffer allocates storage for a new Buffer and initializes it.
func NewBuffer() (b *Buffer) {
	b = new(Buffer)
	b.Init()
	return b
}

func (b *Buffer) initWithCells(c *rawCells, logger *log.Logger) {
	b.cells = c
	b.editor = b.cells
	b.view = b.cells

	if logger != nil {
		cellLogger := newLogger(b.view, b.cells, logger)
		b.view = cellLogger
		b.editor = cellLogger
	}

	// setup the root publisher as the deepest Editor
	b.rootPub = newPublisher(b.editor)
	b.undoer = newUndoer(b.rootPub)
	b.editor = b.undoer
	// setup the usage publisher at the shallowest Editor
	b.usagePub = newPublisher(b.editor)
	b.editor = b.usagePub
	b.safew = safeEditor{editor: b.editor, view: b.view}

	b.selector.view = b.view
}

// InitWithTabspaces initializes this Buffer with
func (b *Buffer) InitWithTabspaces(tabspaces int) {
	cells := new(rawCells)
	cells.init(tabspaces)
	b.initWithCells(cells, nil)
}

// Init initializes this Buffer with the default configuration.
func (b *Buffer) Init() {
	b.InitWithTabspaces(defTabSpaces)
}

// WithLogger adds a cell logger which intercepts and logs all
// the calls to the underlying Editor/View.
func (b *Buffer) WithLogger(logger *log.Logger) {
	b.initWithCells(b.cells, logger)
}

// InsertRowAt inserts a new row at given position. If pos is out of bounds,
// this method does not panic; instead, it will fill in the necessary
// rows such that the new row is the last row in the buffer.
func (b *Buffer) InsertRowAt(y int) {
	var at term.Coordinates
	if y == 0 {
		at = term.Coordinates{Y: y}
	} else if y > b.Rows() {
		// insert fill-in feature takes care of inserting a row up to y
		at = term.Coordinates{Y: y - 1}
	} else {
		// insert new line at the end of previous row
		at = term.Coordinates{Y: y - 1, X: b.Columns(y - 1)}
	}
	b.editor.Edit(at, at, "\n")
}

// Insert inserts a rune in the given position and shift the cells to the right
func (b *Buffer) Insert(pos term.Coordinates, r rune) (next term.Coordinates) {
	_, next, _ = b.editor.Edit(pos, pos, string(r))
	return
}

// InsertWithAttr writes str and gives it attr term.Attributes.
func (b *Buffer) InsertWithAttr(
	pos term.Coordinates, r rune, attr term.Attributes,
) (next term.Coordinates) {
	next = b.Insert(pos, r)

	switch r {
	case '\n', '\t':
		return
	}
	cells := b.RawCells()
	cells[pos.Y][pos.X].Bg = attr.Bg
	cells[pos.Y][pos.X].Fg = attr.Fg

	return
}

// InsertStringWithAttr inserts str with the given attr as the background
// and foreground cell term.Attributes.
func (b *Buffer) InsertStringWithAttr(
	at term.Coordinates, str string, attr term.Attributes,
) (from, until term.Coordinates) {
	from, until = b.InsertString(at, str)
	cells, _ := b.Select(from, until)
	for y, row := range cells {
		for x := range row {
			cells[y][x].Bg = attr.Bg
			cells[y][x].Fg = attr.Fg
		}
	}
	return
}

// DeleteRow deletes the row at term.Coordinates.Y
func (b *Buffer) DeleteRow(y int) (ok bool) {
	if ok = y < b.view.Rows(); !ok {
		return
	}
	var from, to term.Coordinates
	if y == 0 {
		from = term.Coordinates{Y: y}
		to = term.Coordinates{Y: y + 1}
	} else {
		from = term.Coordinates{Y: y - 1, X: b.view.Columns(y - 1)}
		to = term.Coordinates{Y: y, X: b.view.Columns(y)}
	}
	_, _, old := b.editor.Edit(from, to, "")
	ok = old != ""
	return
}

// TruncateRowFrom truncates the row at term.Coordinates.Y starting
// from term.Coordinates.X
func (b *Buffer) TruncateRowFrom(from term.Coordinates) (ok bool) {
	to := term.Coordinates{Y: from.Y}
	from, to, ok = fromToInBounds(b.view, from, to)
	if !ok {
		return
	}
	cols := b.view.Columns(to.Y)
	if cols == 0 {
		ok = false
		return
	}
	to.X = cols
	b.editor.Edit(from, to, "")
	return
}

// TruncateFrom truncates from the given position to the end of the buffer.
func (b *Buffer) TruncateFrom(from term.Coordinates) (ok bool) {
	to := term.Coordinates{Y: b.view.Rows()}
	from, to, ok = fromToInBounds(b.view, from, to)
	if !ok {
		return
	}
	b.editor.Edit(from, to, "")
	return
}

// ConflateRow will conflate row at index i with the next row
func (b *Buffer) ConflateRow(y int) (ok bool) {
	from := term.Coordinates{Y: y}
	to := term.Coordinates{Y: y + 1}
	from, to, ok = fromToInBounds(b.view, from, to)
	if !ok {
		return
	}
	from.X = b.view.Columns(from.Y)
	b.editor.Edit(from, to, "")
	return
}

// DeleteCell removes the cell at the given position.
// It returns the position at which the current cell (width padding) started,
// if the width was > 1.
func (b *Buffer) DeleteCell(pos term.Coordinates) (term.Coordinates, rune, bool) {
	from := pos
	to := term.Coordinates{X: from.X + 1, Y: from.Y}
	from, to, ok := fromToInBounds(b.view, from, to)
	if !ok {
		return term.Coordinates{}, 0, false
	}

	start, _, str := b.editor.Edit(from, to, "")
	if str == "" {
		return term.Coordinates{}, 0, false
	}

	return start, []rune(str)[0], true
}

// Rows returns the number of rows in the Buffer.
func (b *Buffer) Rows() int {
	return b.view.Rows()
}

// Columns returns the number of cells of row at index y.
func (b *Buffer) Columns(y int) int {
	return b.view.Columns(y)
}

// MaxColumns returns the max number of columns.
func (b *Buffer) MaxColumns() (max int) {
	for i := 0; i < b.Rows(); i++ {
		if col := b.Columns(i); col > max {
			max = col
		}
	}
	return
}

// Cell returns the cell and true or a zero-valued cell and false if there is no
// cell at position.
func (b *Buffer) Cell(pos term.Coordinates) (term.Cell, bool) {
	return b.view.Cell(pos)
}

// RawCells gives clients access to the underlying cell matrix.
func (b *Buffer) RawCells() [][]term.Cell {
	return b.view.RawCells()
}

// InsertString inserts string in the given position and shifts the remaining cells.
// insert never fails: if at is out-of-bounds, this method fills in the rows
// and/or columns of cells with blank spaces.
// It returns the start of the insert 'from', including the filled-in blank spaces
// and where the next logical Insert should go 'until'.
func (b *Buffer) InsertString(at term.Coordinates, str string) (
	from, until term.Coordinates,
) {
	from, until, _ = b.editor.Edit(at, at, str)
	return
}

// Delete removes cells in left-inclusive right-exclusive range
// and returns the corresponding string representation of the cells removed,
// along with the true start of the range, which accounts for padding.
func (b *Buffer) Delete(from, to term.Coordinates) (start term.Coordinates, str string) {
	start, _, str = b.safew.Edit(from, to, "")
	return
}

// Edit removes cells in left-inclusive right-exclusive range (start, end) and
// inserts s at the start of the range. It returns the corresponding string
// representation of the content removed and the true from, to range, which
// accounts for possible padding added or removed.
//
// As opposed to cell.Editor.Edit, this method does not panic if range
// is out of bounds. Instead, it trims the coordinates to be in-bounds or
// simply does nothing and returned str is empty.
func (b *Buffer) Edit(start, end term.Coordinates, s string) (
	from, to term.Coordinates, old string,
) {
	return b.safew.Edit(start, end, s)
}

// DeleteLine deletes the lines starting at from, between from, to and including end.
// See Delete for more information about the return values.
func (b *Buffer) DeleteLine(from, to term.Coordinates) (
	start term.Coordinates, str string,
) {
	from, to = SortFromTo(from, to)
	from.X, to.X = 0, 0
	to.Y++
	from, to, ok := fromToInBounds(b.view, from, to)
	if !ok {
		return
	}
	start, _, str = b.editor.Edit(from, to, "")
	return
}

// DeleteBlock deletes the blocks of cells between from, to. See SelectBlock for more
// information about how DeleteBlock selects the cells to delete.
// See Delete for more information about the return values.
func (b *Buffer) DeleteBlock(from, to term.Coordinates) (
	start term.Coordinates, str string,
) {
	var builder strings.Builder
	b.selector.iterateBlocks(from, to,
		func(i int, from, to term.Coordinates, cells []term.Cell) {
			if i != 0 {
				builder.WriteRune('\n')
			}
			columns := b.Columns(from.Y)
			if columns == 0 || from.X > columns {
				if i == 0 {
					start = term.Coordinates{Y: from.Y}
				}
				return
			}

			if to.X > columns {
				to.X = columns
			}
			blockStart, _, str := b.safew.Edit(from, to, "")

			if i == 0 {
				start = blockStart
			}
			builder.WriteString(str)
		})

	str = builder.String()
	return
}

// Reset resets the contents of this Buffer.
func (b *Buffer) Reset() {
	// make sure that reset is propagated to subscribers.
	b.TruncateFrom(term.Coordinates{})
	b.undoer.reset()
}

func (b *Buffer) Version() int {
	return b.undoer.version
}

// ReadFrom reads data from r until EOF and appends it to the buffer, growing
// the buffer as needed. The return value n is the number of bytes read. Any
// error except io.EOF encountered during the read is also returned.
func (b *Buffer) ReadFrom(r io.Reader) (int64, error) {
	return b.cells.ReadFrom(r)
}

// io.Editor
func (b *Buffer) Write(p []byte) (int, error) {
	nextWrite := nextWrite(b.view)
	b.editor.Edit(nextWrite, nextWrite, string(p))
	return len(p), nil
}

// WriteString writes the given string at the end of the buffer
func (b *Buffer) WriteString(p string) {
	nextWrite := nextWrite(b.view)
	b.editor.Edit(nextWrite, nextWrite, p)
}

// WriteStringWithAttr inserts str with the given attr as the background
// and foreground cell term.Attributes.
func (b *Buffer) WriteStringWithAttr(str string, attr term.Attributes) {
	at := nextWrite(b.view)
	b.InsertStringWithAttr(at, str, attr)
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
func (b *Buffer) Select(from term.Coordinates, to term.Coordinates) (
	[][]term.Cell, bool,
) {
	from, to, ok := fromToInBounds(b.view, from, to)
	if !ok {
		return nil, false
	}
	return b.selector.selectCells(from, to), true
}

// SelectLine returns the lines inside the given coordinates or nil if
// coordinates are out of bounds.
func (b *Buffer) SelectLine(from term.Coordinates, to term.Coordinates) (
	[][]term.Cell, bool,
) {
	from, to, ok := fromToInBounds(b.view, from, to)
	if !ok {
		return nil, false
	}
	return b.selector.selectLine(from, to), true
}

// SelectBlock returns the block of cells inside the given coordinates or nil if
// coordinates are out of bounds.
func (b *Buffer) SelectBlock(from term.Coordinates, to term.Coordinates) (
	[][]term.Cell, bool,
) {
	from, to, ok := fromToInBounds(b.view, from, to)
	if !ok {
		return nil, false
	}
	return b.selector.selectBlock(from, to), true
}

func (b *Buffer) String() string {
	return b.view.String()
}

// Tabspaces returns the number of tabspaces uses to initialized this Buffer.
func (b *Buffer) Tabspaces() int {
	return b.cells.tabspaces
}

// ShiftRowRight shifts row one tab to the right. It returns
// the number of cells that the line was shifted.
func (b *Buffer) ShiftRowRight(row int) int {
	b.Insert(term.Coordinates{Y: row}, '\t')
	return b.Tabspaces()
}

// ShiftRowLeft shifts row one tab to the left. It returns
// the number of cells that the line was shifted.
func (b *Buffer) ShiftRowLeft(row int) (chars int) {
	from, to := term.Coordinates{Y: row}, term.Coordinates{Y: row, X: 1}
	from, to, ok := fromToInBounds(b.view, from, to)
	if !ok {
		return
	}
	origLen := b.Columns(row)
	for chars < b.Tabspaces() {
		c, ok := b.view.Cell(from)
		if !ok {
			return
		}
		switch c.Ch {
		case '\t', '\x00', ' ':
			b.Delete(from, to)
			chars = origLen - b.Columns(row)
		default:
			return
		}
	}
	return
}

// Subscribe subscribes s to all updates to the underlying buffer.
func (b *Buffer) Subscribe(s Subscriber) {
	b.rootPub.Subscribe(s)
}

// Unsubscribe unsubscribes s from updates.
func (b *Buffer) Unsubscribe(s Subscriber) {
	b.rootPub.Unsubscribe(s)
}

// SubscribeUsage subscribes s to direct update calls to this buffer. Unlike Subscribe,
// this method does not capture indirect updates to Buffer, for instance via Undo.
func (b *Buffer) SubscribeUsage(s Subscriber) {
	b.usagePub.Subscribe(s)
}

// UnsubscribeUsage reverses SubscribeUsage.
func (b *Buffer) UnsubscribeUsage(s Subscriber) {
	b.usagePub.Unsubscribe(s)
}

// Height returns the required height if this Buffer was to be drawn on a term.Editor.
func (b *Buffer) Height() int {
	return b.Rows()
}

// Width returns the required width if this Buffer was to be drawn on a term.Editor.
func (b *Buffer) Width() int {
	var ret int
	for _, row := range b.RawCells() {
		if len(row) > ret {
			ret = len(row)
		}
	}
	return ret
}

// View returns this Buffer as a cell.View.
func (b *Buffer) View() View {
	return b.view
}

// Editor returns a cell.Editor that doesn't panic on out-of-bounds calls.
func (b *Buffer) Editor() Editor {
	return b.safew
}

// Size returns the total size in cells of this buffer.
func (b *Buffer) Size() (ret int) {
	for _, row := range b.RawCells() {
		ret += len(row)
	}
	return
}
