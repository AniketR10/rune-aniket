package fractal

import (
	"termbox"
)

const defTabSpaces int = 4

type CellBuf struct {
	cells     [][]termbox.Cell
	Tabspaces int
}

func makeNewRow(rowlen int) (row []termbox.Cell) {
	row = make([]termbox.Cell, rowlen)
	return
}

func (b *CellBuf) insertNewRow(pos Coordinates) {
	sourceRow := b.cells[pos.Y]
	targetY := pos.Y + 1

	// make enough space for one more row
	b.cells = append(b.cells, nil)
	copy(b.cells[targetY:], b.cells[pos.Y:])

	// if not last position, copy the rest of cells to the next row
	if pos.X < len(sourceRow) {
		b.cells[pos.Y] = b.cells[pos.Y][:pos.X]
		b.cells[targetY] = makeNewRow(len(sourceRow[pos.X:]))
		copy(b.cells[targetY], sourceRow[pos.X:])
	} else {
		b.cells[targetY] = makeNewRow(0)
	}
}

func (b *CellBuf) nextWrite() Coordinates {
	// cannot be 0, since we always have at least one row
	y := len(b.cells) - 1
	x := len(b.cells[y])
	return Coordinates{X: x, Y: y}
}

func (b *CellBuf) fillInRows(y int) {
	if b.cells == nil {
		b.Init()
	}
	for y >= len(b.cells) {
		row := makeNewRow(0)
		b.cells = append(b.cells, row)
	}
}

func (b *CellBuf) doInsertAt(pos Coordinates, r rune) {
	// make sure we have enough capacity
	b.cells[pos.Y] = append(b.cells[pos.Y], termbox.Cell{})
	copy(b.cells[pos.Y][pos.X+1:], b.cells[pos.Y][pos.X:])
	b.cells[pos.Y][pos.X] = termbox.Cell{Ch: r}
}

func (b *CellBuf) insertAt(pos Coordinates, r rune) (next Coordinates) {
	switch r {
	case '\n':
		b.insertNewRow(pos)
		next = Coordinates{X: 0, Y: pos.Y + 1}
	case '\t':
		if b.Tabspaces > 0 {
			n := Coordinates{X: pos.X, Y: pos.Y}
			for i := 1; i < b.Tabspaces; i++ {
				n = b.insertAt(n, '\x00')
			}
			b.doInsertAt(n, r)
			next = Coordinates{X: pos.X + b.Tabspaces, Y: pos.Y}
			break
		}
		fallthrough
	default:
		b.doInsertAt(pos, r)
		next = Coordinates{X: pos.X + 1, Y: pos.Y}
	}

	return
}

// Init resets and initializes the CellBuf. Note that clients of CellBuf are not required to call this method.
func (b *CellBuf) Init() {
	b.cells = make([][]termbox.Cell, 1)
	b.cells[0] = makeNewRow(0)
	if b.Tabspaces == 0 {
		b.Tabspaces = defTabSpaces
	}
}

// WriteStr writes the given string at the end of the buffer
func (b *CellBuf) WriteStr(p string) Coordinates {
	if b.cells == nil {
		b.Init()
	}
	c := b.nextWrite()
	for _, r := range p {
		c = b.insertAt(c, r)
	}
	return c
}

// Write writes the given rune at the end of the buffer
func (b *CellBuf) Write(r rune) Coordinates {
	if b.cells == nil {
		b.Init()
	}
	c := b.nextWrite()
	return b.insertAt(c, r)
}

func (b *CellBuf) fillInColumns(pos Coordinates) {
	for pos.X > len(b.cells[pos.Y]) {
		b.cells[pos.Y] = append(b.cells[pos.Y], termbox.Cell{Ch: '\x00'})
	}
}

// WriteAt overwrites the cell at the given position with rune
func (b *CellBuf) WriteAt(pos Coordinates, r rune) {
	b.fillInRows(pos.Y)
	b.fillInColumns(pos)
	b.TruncateCellAt(pos)
	b.insertAt(pos, r)
}

// InsertAt inserts a rune in the given position and shift the cells to the right
func (b *CellBuf) InsertAt(pos Coordinates, r rune) Coordinates {
	b.fillInRows(pos.Y)
	b.fillInColumns(pos)
	return b.insertAt(pos, r)
}

// TruncateLastRow truncates the last row in the buffer
func (b *CellBuf) TruncateLastRow() {
	last := len(b.cells) - 1
	// we need to guarantee that there's always at least one row
	if last == 0 {
		b.TruncateRowFrom(Coordinates{X: 0, Y: 0})
		return
	}

	b.cells = b.cells[:last]
}

// TruncateRowAt truncates the row at Coordinates.Y
func (b *CellBuf) TruncateRowAt(i int) (ok bool) {
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
func (b *CellBuf) TruncateRowFrom(pos Coordinates) {
	b.cells[pos.Y] = b.cells[pos.Y][:pos.X]
}

// TruncateFrom truncates from the given position to the end of the buffer
func (b *CellBuf) TruncateFrom(pos Coordinates) {
	// remove until we have only row Y
	if pos.Y+1 < len(b.cells) {
		b.cells = b.cells[:pos.Y+1]
	}
	// remove the remaining characters in row Y
	b.TruncateRowFrom(pos)
}

// ConflateRow will conflate row at index i with the next row
func (b *CellBuf) ConflateRow(i int) (ok bool) {
	// if last row or beyond can't conflate
	if i >= len(b.cells)-1 {
		return
	}
	for _, c := range b.cells[i+1] {
		b.cells[i] = append(b.cells[i], termbox.Cell{Ch: c.Ch})
	}
	b.TruncateRowAt(i + 1)
	ok = true
	return
}

// TruncateCellAt truncates the cell at the given position and shifts the cells on the right to the left.
// It returns the position of the next available cell.
func (b *CellBuf) TruncateCellAt(pos Coordinates) (next Coordinates, orig termbox.Cell, ok bool) {
	if pos.Y >= len(b.cells) || pos.X >= len(b.cells[pos.Y]) {
		return
	}
	orig = b.cells[pos.Y][pos.X]
	lastIdx := len(b.cells[pos.Y]) - 1
	offset := 0

	if orig.Ch == '\t' {
		offset = b.Tabspaces - 1
	}

	x := pos.X - offset

	if pos.X < lastIdx {
		copy(b.cells[pos.Y][pos.X-offset:], b.cells[pos.Y][pos.X+1:])
	}
	b.cells[pos.Y] = b.cells[pos.Y][:lastIdx-offset]
	ok = true
	next = Coordinates{X: x, Y: pos.Y}
	return
}

// RowLen returns the number of cells of row at index i
func (b *CellBuf) RowLen(i int) (j int, ok bool) {
	if i >= len(b.cells) {
		return
	}
	j = len(b.cells[i])
	ok = true
	return
}

// Rows returns the number of rows in the buffer
func (b *CellBuf) Rows() int {
	return len(b.cells)
}

func (b *CellBuf) String() string {
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
