package fractal

import (
	"bufio"
	"fmt"
	"io"
	"math"

	"github.com/nsf/termbox-go"
)

const defTabSpaces int = 4
const defColumnCap int = 64
const defRowCap int = 128

// A Buffer is a variable-sized matrix of cells.
// The zero value for Buffer is ready to use.
type Buffer struct {
	cells     [][]termbox.Cell
	tabspaces int
}

func makeNewRow(length, capacity int) (row []termbox.Cell) {
	row = make([]termbox.Cell, length, capacity)
	return
}

func assertCoordinates(pos Coordinates) {
	if pos.X < 0 || pos.Y < 0 {
		panic(fmt.Sprintf("invalid coordinates: %+v", pos))
	}
}

func (b *Buffer) insertNewRow(pos Coordinates) {
	assertCoordinates(pos)
	sourceRow := b.cells[pos.Y]
	targetY := pos.Y + 1

	// make enough space for one more row
	b.cells = append(b.cells, nil)
	copy(b.cells[targetY:], b.cells[pos.Y:])

	// if not last position, copy the rest of cells to the next row
	if pos.X < len(sourceRow) {
		b.cells[pos.Y] = b.cells[pos.Y][:pos.X]
		length := len(sourceRow[pos.X:])
		b.cells[targetY] = makeNewRow(length, length)
		copy(b.cells[targetY], sourceRow[pos.X:])
	} else {
		b.cells[targetY] = makeNewRow(0, defColumnCap)
	}
}

// InsertRowAt inserts a new row at given position. If pos is out of bounds,
// this method does not panic; instead, it will fill in the necessary
// rows such that the new row is the last row in the buffer.
func (b *Buffer) InsertRowAt(i int) {
	b.fillInRows(i)
	b.insertNewRow(Coordinates{Y: i, X: 0})
}

func (b *Buffer) nextWrite() Coordinates {
	// cannot be 0, since we always have at least one row
	y := len(b.cells) - 1
	x := len(b.cells[y])
	return Coordinates{X: x, Y: y}
}

func (b *Buffer) fillInRows(y int) {
	if b.cells == nil {
		b.Reset()
	}
	for y >= len(b.cells) {
		row := makeNewRow(0, defColumnCap)
		b.cells = append(b.cells, row)
	}
}

func (b *Buffer) doInsertAt(pos Coordinates, r rune) {
	// make sure we have enough capacity
	b.cells[pos.Y] = append(b.cells[pos.Y], termbox.Cell{})
	copy(b.cells[pos.Y][pos.X+1:], b.cells[pos.Y][pos.X:])
	b.cells[pos.Y][pos.X] = termbox.Cell{Ch: r}
}

func (b *Buffer) insertAt(pos Coordinates, r rune) (next Coordinates) {
	switch r {
	case '\n':
		b.insertNewRow(pos)
		next = Coordinates{X: 0, Y: pos.Y + 1}
	case '\t':
		if b.tabspaces > 0 {
			b.insertTabSpaces(pos)
			next = Coordinates{X: pos.X + b.tabspaces, Y: pos.Y}
			break
		}
		fallthrough
	default:
		b.doInsertAt(pos, r)
		next = Coordinates{X: pos.X + 1, Y: pos.Y}
	}

	return
}

// Init initializes this Buffer with the given tabspaces config and resets its contents.
func (b *Buffer) Init(tabspaces int) {
	b.tabspaces = tabspaces
	b.Reset()
}

// Reset resets the contents of this cellbuf.
func (b *Buffer) Reset() {
	b.cells = make([][]termbox.Cell, 1, defRowCap)
	b.cells[0] = makeNewRow(0, defColumnCap)
	if b.tabspaces == 0 {
		b.tabspaces = defTabSpaces
	}
}

func (b *Buffer) insertTabSpaces(pos Coordinates) {
	n := Coordinates{X: pos.X, Y: pos.Y}
	for i := 1; i < b.tabspaces; i++ {
		n = b.insertAt(n, '\x00')
	}
	b.doInsertAt(n, '\t')
}

func (b *Buffer) appendString(p string) {
	rowY := b.nextWrite().Y
	for _, r := range p {
		switch r {
		case '\n':
			b.cells = append(b.cells, makeNewRow(0, defColumnCap))
			rowY++
		case '\t':
			if b.tabspaces > 0 {
				for i := 1; i < b.tabspaces; i++ {
					b.cells[rowY] = append(b.cells[rowY], termbox.Cell{})
				}
				b.cells[rowY] = append(b.cells[rowY], termbox.Cell{Ch: '\t'})
				break
			}
			fallthrough
		default:
			b.cells[rowY] = append(b.cells[rowY], termbox.Cell{Ch: r})
		}
	}
}

// WriteString writes the given string at the end of the buffer
func (b *Buffer) WriteString(p string) Coordinates {
	if b.cells == nil {
		b.Reset()
	}
	b.appendString(p)
	return b.nextWrite()
}

// WriteRune writes the given rune at the end of the buffer
func (b *Buffer) WriteRune(r rune) Coordinates {
	if b.cells == nil {
		b.Reset()
	}
	c := b.nextWrite()
	return b.insertAt(c, r)
}

func (b *Buffer) fillInColumns(pos Coordinates) {
	for pos.X > len(b.cells[pos.Y]) {
		b.cells[pos.Y] = append(b.cells[pos.Y], termbox.Cell{Ch: ' '})
	}
}

// WriteAt overwrites the cell at the given position with rune
func (b *Buffer) WriteAt(pos Coordinates, r rune) {
	assertCoordinates(pos)
	b.fillInRows(pos.Y)
	b.fillInColumns(pos)
	b.TruncateCellAt(pos)
	b.insertAt(pos, r)
}

// InsertAt inserts a rune in the given position and shift the cells to the right
func (b *Buffer) InsertAt(pos Coordinates, r rune) Coordinates {
	assertCoordinates(pos)
	b.fillInRows(pos.Y)
	b.fillInColumns(pos)
	return b.insertAt(pos, r)
}

// TruncateLastRow truncates the last row in the buffer
func (b *Buffer) TruncateLastRow() {
	last := len(b.cells) - 1
	// we need to guarantee that there's always at least one row
	if last <= 0 {
		b.TruncateRowFrom(Coordinates{X: 0, Y: 0})
		return
	}

	b.cells = b.cells[:last]
}

// TruncateRowAt truncates the row at Coordinates.Y
func (b *Buffer) TruncateRowAt(i int) (ok bool) {
	last := len(b.cells) - 1
	if i > last {
		return
	}

	if i != last {
		copy(b.cells[i:], b.cells[i+1:])
	}
	b.TruncateLastRow()
	ok = true
	return
}

// TruncateRowFrom truncates the row at Coordinates.Y starting from Coordinates.X
func (b *Buffer) TruncateRowFrom(pos Coordinates) (ok bool) {
	assertCoordinates(pos)
	if pos.Y >= len(b.cells) || pos.X >= len(b.cells[pos.Y]) {
		return
	}
	ok = true
	b.cells[pos.Y] = b.cells[pos.Y][:pos.X]
	return
}

// TruncateFrom truncates from the given position to the end of the buffer.
func (b *Buffer) TruncateFrom(pos Coordinates) (ok bool) {
	assertCoordinates(pos)
	// remove until we have only row Y
	if pos.Y+1 < len(b.cells) {
		ok = true
		b.cells = b.cells[:pos.Y+1]
	}
	// remove the remaining characters in row Y
	return b.TruncateRowFrom(pos) || ok
}

// ConflateRow will conflate row at index i with the next row
func (b *Buffer) ConflateRow(i int) (ok bool) {
	if i < 0 || i >= len(b.cells)-1 {
		return
	}
	for _, c := range b.cells[i+1] {
		b.cells[i] = append(b.cells[i], termbox.Cell{Ch: c.Ch})
	}
	b.TruncateRowAt(i + 1)
	ok = true
	return
}

func (b *Buffer) truncateCellAt(pos Coordinates) {
	lastIdx := len(b.cells[pos.Y]) - 1
	if pos.X < lastIdx {
		copy(b.cells[pos.Y][pos.X:], b.cells[pos.Y][pos.X+1:])
	}
	b.cells[pos.Y] = b.cells[pos.Y][:lastIdx]
	return
}

func (b *Buffer) truncateTabPadding(n int, pos Coordinates) int {
	for ; n < b.tabspaces; n++ {
		pos.X--
		b.truncateCellAt(Coordinates{Y: pos.Y, X: pos.X})
	}
	return n
}

// TruncateCellAt truncates the cell at the given position.
// It returns the cell truncated along with the number of cells truncated
// because if a tab cell was truncated the tab padding is truncated along with it.
func (b *Buffer) TruncateCellAt(pos Coordinates) (orig termbox.Cell, n int) {
	assertCoordinates(pos)
	if pos.Y >= len(b.cells) || pos.X >= len(b.cells[pos.Y]) {
		return
	}
	orig = b.cells[pos.Y][pos.X]
	n++
	b.truncateCellAt(pos)

	// truncate tab padding on the left
	if orig.Ch == '\t' {
		n = b.truncateTabPadding(n, pos)
		return
	}

	// truncate tab padding on the left and on the right
	if orig.Ch == '\x00' {
		for orig.Ch != '\t' {
			orig = b.cells[pos.Y][pos.X]
			b.truncateCellAt(Coordinates{Y: pos.Y, X: pos.X})
			n++
		}
		n = b.truncateTabPadding(n, pos)
	}
	return
}

// RowLen returns the number of cells of row at index i
func (b *Buffer) RowLen(i int) (j int, ok bool) {
	if i >= len(b.cells) {
		return
	}
	j = len(b.cells[i])
	ok = true
	return
}

// Rows returns the number of rows in the buffer
func (b *Buffer) Rows() int {
	return len(b.cells)
}

func (b *Buffer) String() string {
	s := make([]rune, 0)
	for _, r := range b.cells {
		for _, c := range r {
			if c.Ch != '\x00' {
				s = append(s, c.Ch)
			}
		}
		s = append(s, '\n')
	}
	// trim last newline
	l := len(s) - 1
	if l >= 0 {
		s = s[:l]
	}
	return string(s)
}

func sortFromTo(from Coordinates, to Coordinates) (Coordinates, Coordinates) {
	if from.X > to.X {
		temp := from.X
		from.X = to.X
		to.X = temp
	}
	if from.Y > to.Y {
		temp := from.Y
		from.Y = to.Y
		to.Y = temp
	}
	return from, to
}

// Select returns the cells inside the given coordinates or nil if coordinates are out of bounds.
func (b *Buffer) Select(from Coordinates, to Coordinates) (res [][]termbox.Cell) {
	res = make([][]termbox.Cell, 0)

	from, to = sortFromTo(from, to)

	for from.Y < to.Y && from.Y < len(b.cells) {
		x := int(math.Min(float64(from.X), float64(len(b.cells[from.Y]))))
		res = append(res, b.cells[from.Y][x:])
		from.X = 0
		from.Y++
	}

	if from.Y >= len(b.cells) {
		return
	}

	fromX := int(math.Min(float64(from.X), float64(len(b.cells[from.Y]))))
	toX := int(math.Min(float64(to.X+1), float64(len(b.cells[from.Y]))))
	res = append(res, b.cells[from.Y][fromX:toX])
	return
}

// SelectLine returns the lines inside the given coordinates or nil if coordinates are out of bounds.
func (b *Buffer) SelectLine(from Coordinates, to Coordinates) (res [][]termbox.Cell) {
	res = make([][]termbox.Cell, 0)

	from, to = sortFromTo(from, to)

	for from.Y <= to.Y && from.Y < len(b.cells) {
		res = append(res, b.cells[from.Y][:])
		from.Y++
	}

	return
}

// SelectBlock returns the block of cells inside the given coordinates or nil if coordinates are out of bounds.
func (b *Buffer) SelectBlock(from Coordinates, to Coordinates) (res [][]termbox.Cell) {
	res = make([][]termbox.Cell, 0)

	from, to = sortFromTo(from, to)

	for from.Y <= to.Y && from.Y < len(b.cells) {
		maxy := float64(len(b.cells[from.Y]))
		xfrom := int(math.Min(maxy, float64(from.X)))
		xto := int(math.Min(maxy, float64(to.X+1)))
		res = append(res, b.cells[from.Y][xfrom:xto])
		from.Y++
	}

	return
}

// RawCells gives clients access to the underlying cell matrix.
func (b *Buffer) RawCells() [][]termbox.Cell {
	if b.cells == nil {
		b.Reset()
	}
	return b.cells
}

// ResetAttr resets all the attributes of the underlying cell matrix.
func (b *Buffer) ResetAttr() {
	for y, r := range b.cells {
		for x := range r {
			b.cells[y][x].Fg, b.cells[y][x].Bg = 0, 0
		}
	}
}

// SetAttr overwrites the background and foreground attributes of cell at position.
func (b *Buffer) SetAttr(pos Coordinates, fg, bg termbox.Attribute) {
	assertCoordinates(pos)
	b.fillInRows(pos.Y)
	b.fillInColumns(pos)
	if len(b.cells[pos.Y]) == pos.X {
		b.insertAt(pos, ' ')
	}
	b.cells[pos.Y][pos.X].Bg = bg
	b.cells[pos.Y][pos.X].Fg = fg
}

// RowLastIdx returns the width of row i.
func (b *Buffer) RowLastIdx(y int) (x int, ok bool) {
	if y < 0 || y >= b.Rows() {
		return
	}

	ok = true
	if len := len(b.cells[y]); len > 0 {
		x = len - 1
	}
	return
}

// ReadFrom reads data from r until EOF and appends it to the buffer, growing
// the buffer as needed. The return value n is the number of bytes read. Any
// error except io.EOF encountered during the read is also returned.
func (b *Buffer) ReadFrom(r io.Reader) (n int64, err error) {
	if b.cells == nil {
		b.Reset()
	}
	rowY := b.nextWrite().Y
	reader := bufio.NewReader(r)
	var bytes []byte
	var isPrefix bool
	for {
		bytes, isPrefix, err = reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
		for _, r := range bytes {
			for i := 1; r == '\t' && i < b.tabspaces; i++ {
				b.cells[rowY] = append(b.cells[rowY], termbox.Cell{})
			}
			b.cells[rowY] = append(b.cells[rowY], termbox.Cell{Ch: rune(r)})
		}
		n += int64(len(bytes) + 1)
		if !isPrefix {
			b.cells = append(b.cells, makeNewRow(0, defColumnCap))
			rowY++
		}
	}
}
