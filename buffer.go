package fractal

import (
	"fmt"
)

const defTabSpaces int = 4

// CellBuf represents a buffer of cells. It is optimized for 2D operations with Coordinates
type CellBuf struct {
	cells     [][]Cell
	Tabspaces int
}

func (b *CellBuf) insertNewRow() {
	nlast := make([]Cell, 0, 1)
	b.cells = append(b.cells, nlast)
}

func (b *CellBuf) appendRune(to Cell, r rune) (lo Cell) {
	y, x := to.Y, to.X+1
	lo = Cell{
		Ch:          r,
		Coordinates: Coordinates{X: x, Y: y},
	}

	switch r {
	case '\n':
		b.cells[y] = append(b.cells[y], lo)
		lo.Y++
		lo.X = -1
		b.insertNewRow()
	case '\t':
		lo.Ch = ' '
		for i := 0; i < b.Tabspaces; i++ {
			b.cells[y] = append(b.cells[y], lo)
			lo.X++
		}
		lo.X--
	default:
		b.cells[y] = append(b.cells[y], lo)
	}
	return lo
}

func (b *CellBuf) lastCell() (Cell, bool) {
	// cannot be 0, since we always have at least one row
	y := len(b.cells) - 1

	// can be 0, since we can have a row with no cells
	x := len(b.cells[y]) - 1
	if x < 0 {
		return Cell{}, false
	}
	return b.cells[y][x], true
}

func (b *CellBuf) fillIn(pos Coordinates) {
	if b.cells == nil {
		b.Init()
	}
	// fill in rows
	for diff := pos.Y - len(b.cells); diff >= 0; diff-- {
		row := make([]Cell, 0)
		b.cells = append(b.cells, row)
	}

	// fill in cells
	for x := pos.X; pos.X >= len(b.cells[pos.Y]); x++ {
		b.cells[pos.Y] = append(b.cells[pos.Y], Cell{
			Ch:          ' ',
			Coordinates: Coordinates{X: x, Y: pos.Y},
		})
	}
}

func (b *CellBuf) writeAt(pos Coordinates, r rune) {
	b.cells[pos.Y][pos.X] = Cell{Ch: r, Coordinates: pos}
	if r == '\n' {
		b.insertNewRow()
	}
}

func (b *CellBuf) insertAt(pos Coordinates, r rune) {
	rowLen := len(b.cells[pos.Y])
	// make sure we have enough capacity
	b.cells[pos.Y] = append(b.cells[pos.Y], Cell{})[:rowLen]
	copy(b.cells[pos.Y][pos.X+1:], b.cells[pos.Y][pos.X:])

	b.writeAt(pos, r)
}

// Init resets and initializes the CellBuf. Note that clients of CellBuf are not required to call this method.
func (b *CellBuf) Init() {
	b.cells = make([][]Cell, 1)
	b.cells[0] = make([]Cell, 0)
	if b.Tabspaces == 0 {
		b.Tabspaces = defTabSpaces
	}
}

// WriteStr writes the given string at the end of the buffer
func (b *CellBuf) WriteStr(p string) {
	if b.cells == nil {
		b.Init()
	}
	c, ok := b.lastCell()
	if !ok {
		c = Cell{Coordinates: Coordinates{X: -1, Y: 0}}
	}
	for _, r := range p {
		c = b.appendRune(c, r)
	}
	return
}

// Write writes the given rune at the end of the buffer
func (b *CellBuf) Write(r rune) {
	if b.cells == nil {
		b.Init()
	}
	if c, ok := b.lastCell(); ok {
		b.appendRune(c, r)
	} else {
		b.appendRune(Cell{Coordinates: Coordinates{X: -1, Y: 0}}, r)
	}
}

// WriteAt overwrites the cell at the given position with rune
func (b *CellBuf) WriteAt(pos Coordinates, r rune) {
	b.fillIn(pos)
	b.writeAt(pos, r)
}

// InsertAt inserts a rune in the given position and shift the cells to the right
func (b *CellBuf) InsertAt(pos Coordinates, r rune) {
	b.fillIn(pos)
	b.insertAt(pos, r)
}

// Truncate truncates the last n cells of the buffer
func (b *CellBuf) Truncate(n int) (t int) {
	var lrowi int
	var lrowlen int
	rows := len(b.cells)
	for {
		lrowi = rows - 1
		if lrowi < 0 {
			return
		}
		lrowlen = len(b.cells[lrowi])
		if t+lrowlen >= n {
			break
		}
		t += lrowlen
		b.cells = b.cells[:lrowi]
		rows--
	}

	b.cells[lrowi] = b.cells[lrowi][:lrowlen+t-n]

	return
}

// TruncateLastRow truncates the last row in the buffer
func (b *CellBuf) TruncateLastRow() {
	last := len(b.cells) - 1
	// we need to guarantee that there's always at least one row
	if last < 0 {
		b.TruncateRowFrom(Coordinates{X: 0, Y: 0})
		return
	}

	b.cells = b.cells[:last]
	b.rewriteCoordinates(Coordinates{X: 0, Y: last - 1}, b.cells)
	// remove newline char from previous row
	if c, ok := b.lastCell(); ok && c.Ch == '\n' {
		b.TruncateCellAt(c.Coordinates)
	}
}

func (b *CellBuf) rewriteCoordinates(from Coordinates, cells [][]Cell) {
	for _, row := range cells {
		for _, c := range row {
			b.cells[from.Y][from.X] = Cell{
				Ch:          c.Ch,
				Coordinates: Coordinates{X: from.X, Y: from.Y},
			}
			from.X++
		}
		from.X = 0
		from.Y++
	}
}

// TruncateRowAt truncates the row at Coordinates.Y
func (b *CellBuf) TruncateRowAt(pos Coordinates) {
	last := len(b.cells) - 1
	if pos.Y > last {
		l, _ := b.lastCell()
		panic(fmt.Sprintf("index out of bounds: trying to truncate beyond last cell %+v", l))
	} else if pos.Y != last {
		copy(b.cells[pos.Y:], b.cells[pos.Y+1:])
	}
	b.TruncateLastRow()
}

// TruncateRowFrom truncates the row at Coordinates.Y starting from Coordinates.X
func (b *CellBuf) TruncateRowFrom(pos Coordinates) {
	b.cells[pos.Y] = b.cells[pos.Y][:pos.X]
	// keep newline if row was in the middle of the buffer
	if len(b.cells)-1 != pos.Y {
		b.cells[pos.Y] = append(b.cells[pos.Y], Cell{Ch: '\n', Coordinates: pos})
	}
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

// TruncateCellAt truncates the cell at the given position and shifts the cells on the right to the left
func (b *CellBuf) TruncateCellAt(pos Coordinates) (orig Cell) {
	orig = b.cells[pos.Y][pos.X]
	lastIdx := len(b.cells[pos.Y]) - 1
	if pos.X < lastIdx {
		copy(b.cells[pos.Y][pos.X:], b.cells[pos.Y][pos.X+1:])
	}
	b.cells[pos.Y] = b.cells[pos.Y][:lastIdx]

	// move cells from next row to this row
	if orig.Ch == '\n' && pos.Y < len(b.cells)-1 {
		x := pos.X
		for _, c := range b.cells[pos.Y+1] {
			b.cells[pos.Y] = append(b.cells[pos.Y], Cell{
				Ch:          c.Ch,
				Coordinates: Coordinates{X: x, Y: pos.Y},
			})
			x++
		}
		b.TruncateLastRow()
	}
	b.rewriteCoordinates(Coordinates{X: 0, Y: pos.Y}, b.cells[pos.Y:pos.Y+1])
	return orig
}

// RowLen returns the number of cells of row at index i
func (b *CellBuf) RowLen(i int) int {
	return len(b.cells[i])
}

// Len returns the total number of cells in the buffer
func (b *CellBuf) Len() (n int) {
	for _, row := range b.cells {
		n += len(row)
	}
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
			s = append(s, c.Ch)
		}
	}
	return string(s)
}
