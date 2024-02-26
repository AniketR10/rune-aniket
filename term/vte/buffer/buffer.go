package buffer

import (
	"math"

	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/term/vte/parser"
)

// Buffer implements a vte terminal buffer by wrapping a cell.Buffer
// and implementing vte buffer manipulation semantics.
type Buffer struct {
	Cells        cell.Buffer
	width        int
	height       int
	topMargin    int // start of scrollable region
	bottomMargin int // end of scrollable region

	savedCursor CursorState
	cursor      CursorState
}

// CursorState holds the state of the cursor.
type CursorState struct {
	position term.Coordinates
	attr     term.Attributes
	hidden   bool // hidden flag on cursor attrs, not cursor itself
	Charsets map[parser.CharsetIndex]parser.StandardCharset
}

// New allocates storage for a new Buffer and initializes it.
func New() *Buffer {
	ret := new(Buffer)
	ret.width = 1
	ret.height = 1
	ret.topMargin = 0
	ret.bottomMargin = ret.height
	ret.cursor = CursorState{
		Charsets: make(map[parser.CharsetIndex]parser.StandardCharset),
	}
	ret.Cells.Init()
	ret.resetLinesTrim(0, ret.height, true, ' ')

	return ret
}

// Resize resizes this Buffer and resets the vertical margins.
func (b *Buffer) Resize(width, height int) {
	b.width = width
	b.height = height
	b.resetLinesTrim(0, height, true, ' ')
	b.SetScrollableRegion(0, 0, true)
}

// Insert inserts a new character at the cursor position, shifting right
// all the cells to the right of the cursor. It does not extend the number columns
// in the buffer, as it should always be capped at exactly b.Width(), set by
// the previous call to Resize.
func (b *Buffer) Insert(c rune, width int, charset parser.CharsetIndex) {
	b.Cells.Insert(b.cursor.position, ' ')
	b.Write(c, width, charset)
	columns := b.Cells.Columns(b.cursor.position.Y)
	if columns > b.width {
		from := term.Coordinates{Y: b.cursor.position.Y, X: b.width}
		to := term.Coordinates{Y: b.cursor.position.Y, X: columns}
		b.Cells.Delete(from, to)
	}
}

// Write writes the given character with the given width to the cell
// at the current cursor position.
func (b *Buffer) Write(c rune, width int, charset parser.CharsetIndex) {
	if charset, ok := b.cursor.Charsets[charset]; ok {
		c = charset.Map(c)
	}
	if b.cursor.hidden {
		c = ' '
	}
	cell := b.CellAt(b.cursor.position)
	if cell == nil {
		b.Insert(c, width, charset)
		return
	}
	cell.Ch = c
	cell.Attributes = b.cursor.attr
	cell.Width = width
}

// ResetCells erases all the cells from start to end, on the current
// cursor line. The start to end range is left inclusive, right exclusive.
func (b *Buffer) ResetCells(start, end int) {
	b.ResetCellsAt(b.cursor.position.Y, start, end, ' ')
}

// ResetCellsAt erases all the cells from start to end, at the given line,
// The start to end range is left inclusive, right exclusive.
func (b *Buffer) ResetCellsAt(y int, start, end int, with rune) {
	cells := b.Cells.RawCells()

	// ensure there are enough columns
	if end > b.Cells.Columns(y) {
		b.Cells.Insert(term.Coordinates{Y: y, X: end - 1}, with)
	}

	for x := start; x < end; x++ {
		cells[y][x] = term.Cell{
			Width: 1,
			Ch:    with,
			Attributes: term.Attributes{
				Bg: b.cursor.attr.Bg,
			},
		}
	}
}

// ResetLines erases all the lines from start to end.
// The start to end range is left inclusive, right exclusive.
func (b *Buffer) ResetLines(start, end int) {
	b.ResetLinesWith(start, end, ' ')
}

// ResetLinesWith erases all the lines from start to end,
// using with and the default attributes as the new content.
// The start to end range is left inclusive, right exclusive.
func (b *Buffer) ResetLinesWith(start, end int, with rune) {
	b.resetLinesTrim(start, end, false, with)
}

// Delete deletes the the given number of cells, shifting left
// all the cells to the right of the cursor.
func (b *Buffer) Delete(count int) {
	if count <= 0 {
		return
	}
	pos := b.cursor.position
	columns := b.Cells.Columns(pos.Y)
	count = int(math.Min(
		float64(count),
		float64(columns),
	))

	cells := b.Cells.RawCells()
	copy(cells[pos.Y][pos.X:], cells[pos.Y][pos.X+count:])
	cells[pos.Y] = cells[pos.Y][:count]

	// reset cells that were deleted
	b.ResetCells(columns-count, columns)
}

// SetCursorPosition updates the cursor position
func (b *Buffer) SetCursorPosition(c term.Coordinates, relative bool) {
	var yOffset, yMax int
	if relative {
		yOffset = b.topMargin
		yMax = b.bottomMargin - 1
	} else {
		yMax = b.height - 1
	}
	b.cursor.position.X = int(math.Max(float64(c.X), 0))
	b.cursor.position.Y = int(math.Max(math.Min(float64(c.Y+yOffset), float64(yMax)), 0))
}

// ScrollDown scrolls down the scrollable region set by SetScrollableRegion by count of lines
func (b *Buffer) ScrollDown(start, end, count int) {
	if end-start <= count {
		b.ResetLines(start, end)
		return
	}

	for y := end - 1; y >= start+count; y-- {
		b.swapLine(y, y-count)
	}

	b.ResetLines(start, start+count)
}

// ScrollUp scrolls up the scrollable region set by SetScrollableRegion by count of lines
func (b *Buffer) ScrollUp(start, end, count int) {
	if end-start <= count {
		b.ResetLines(start, end)
		return
	}

	for y := start; y < end-count; y++ {
		b.swapLine(y, y+count)
	}

	b.ResetLines(end-count, end)
}

// SetScrollableRegion sets the start and end of the scrollable area.
func (b *Buffer) SetScrollableRegion(top int, bottom int, end bool) {
	b.topMargin = int(math.Min(float64(top), float64(b.height)))
	if end {
		b.bottomMargin = b.height
	} else {
		b.bottomMargin = int(math.Min(float64(bottom), float64(b.height)))
	}
}

// BottomMargin returns the bottom margin, set by SetVerticalMargins.
func (b *Buffer) BottomMargin() int {
	return b.bottomMargin
}

// TopMargin returns the bottom margin, set by SetVerticalMargins.
func (b *Buffer) TopMargin() int {
	return b.topMargin
}

// Height returns the height set by Resize.
func (b *Buffer) Height() int {
	return b.height
}

// Width returns the height set by Resize.
func (b *Buffer) Width() int {
	return b.width
}

// Columns returns the columns of the given line.
func (b *Buffer) Columns(line int) int {
	return b.Cells.Columns(line)
}

// CellAt returns the cell at the given position or nil
// if there's no cell at the given position.
func (b *Buffer) CellAt(pos term.Coordinates) *term.Cell {
	// Do not use Height, or intended number of screen lines here:
	// there might be a significant latency betwen resizing and upserting cells.
	// This effectively prevents Insert(pos)=ok then CellAt(pos)=nil
	cells := b.Cells.RawCells()
	if pos.Y >= len(cells) {
		return nil
	}
	if pos.X >= len(cells[pos.Y]) {
		return nil
	}
	return &b.Cells.RawCells()[pos.Y][pos.X]
}

// SaveCursor saves the current cursor state to be restored
// later by RestoreCursor.
func (b *Buffer) SaveCursor() {
	b.SetSavedCursor(b.CloneCursor())
}

// CloneCursor clones the current cursor state and returns it.
func (b *Buffer) CloneCursor() (ret CursorState) {
	ret.attr = b.cursor.attr
	ret.position = b.cursor.position
	ret.Charsets = make(map[parser.CharsetIndex]parser.StandardCharset)
	for k, v := range b.cursor.Charsets {
		ret.Charsets[k] = v
	}
	return ret
}

// SetCursor sets the current cursor state to c.
func (b *Buffer) SetCursor(c CursorState) {
	b.cursor = c
}

// SetSavedCursor sets the saved cursor, to be restored
// later by RestoreCursor.
func (b *Buffer) SetSavedCursor(c CursorState) {
	b.savedCursor = c
}

// RestoreCursor sets the cursor to the previously stored
// cursor via SaveCursor or SetSavedCursor.
func (b *Buffer) RestoreCursor() {
	b.SetCursor(b.savedCursor)
}

// Cursor returns the current CursorState.
func (b *Buffer) Cursor() CursorState {
	return b.cursor
}

// CursorPosition returns the current cursor position.
func (b *Buffer) CursorPosition() term.Coordinates {
	return b.cursor.position
}

// SetHiddenCursor marks as hidden the current cursor attributes.
func (b *Buffer) SetHiddenCursor(hidden bool) {
	b.cursor.hidden = hidden
}

// CursorAttributes returns the current cursor attributes.
func (b *Buffer) CursorAttributes() term.Attributes {
	return b.cursor.attr
}

// SetCursorAttributes sets the default cursor attributes.
func (b *Buffer) SetCursorAttributes(attr term.Attributes) {
	b.cursor.attr = attr
}

// ConfigureCharset configures the given charset index to use charset.
func (b *Buffer) ConfigureCharset(
	index parser.CharsetIndex, charset parser.StandardCharset,
) {
	b.cursor.Charsets[index] = charset
}

// MaxColumns returns the max columns of the underlying cell.Buffer.
func (b *Buffer) MaxColumns() int {
	return b.Cells.MaxColumns()
}

func (b *Buffer) resetLinesTrim(start, end int, trim bool, with rune) {
	end = int(math.Min(float64(b.height), float64(end)))
	if start < 0 || end <= 0 || start >= end {
		return
	}
	// ensure there are enough rows
	if end > b.Cells.Rows() {
		b.Cells.Insert(term.Coordinates{Y: end - 1}, with)
	} else if end < b.Cells.Rows() && trim {
		b.Cells.TruncateFrom(term.Coordinates{Y: end - 1})
	}

	for y := start; y < end; y++ {
		columns := b.Cells.Columns(y)
		if columns > b.width && trim {
			from := term.Coordinates{Y: y, X: b.width}
			to := term.Coordinates{Y: y, X: columns}
			b.Cells.Delete(from, to)
		}
		b.ResetCellsAt(y, 0, b.width, with)
	}
}

func (b *Buffer) swapLine(i, j int) {
	cells := b.Cells.RawCells()
	temp := cells[i]
	cells[i] = cells[j]
	cells[j] = temp
}
