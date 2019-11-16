package cell

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/ernestrc/fractal/term"
)

const defTabSpaces int = 4
const defColumnCap int = 64
const defRowCap int = 128

// A Cells is a matrix of cells. The zero value for Cells is ready to use.
type Cells struct {
	cells     [][]term.Cell
	tabspaces int
}

// Init initializes this Cells with the given tabspaces config and resets its contents.
func (b *Cells) Init(tabspaces int) {
	b.tabspaces = tabspaces
	b.Reset()
}

// Reset resets the contents of this cellbuf.
func (b *Cells) Reset() {
	b.cells = make([][]term.Cell, 1, defRowCap)
	b.cells[0] = makeNewRow(0, defColumnCap)
	if b.tabspaces == 0 {
		b.tabspaces = defTabSpaces
	}
}

func assertValidCoords(pos term.Coordinates) {
	if pos.X < 0 || pos.Y < 0 {
		panic(fmt.Sprintf("invalid coordinates: %+v", pos))
	}
}

func (b *Cells) assertCordsInBounds(pos term.Coordinates) {
	assertValidCoords(pos)
	if pos.Y >= b.Rows() || pos.X > len(b.cells[pos.Y]) {
		panic(fmt.Sprintf("Coordinates out of bounds: %+v", pos))
	}
}

func makeNewRow(length, capacity int) (row []term.Cell) {
	row = make([]term.Cell, length, capacity)
	return
}

func (b *Cells) insertNewRow(pos term.Coordinates) {
	assertValidCoords(pos)
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

func (b *Cells) doInsertAt(pos term.Coordinates, r rune) {
	// make sure we have enough capacity
	b.cells[pos.Y] = append(b.cells[pos.Y], term.Cell{})
	copy(b.cells[pos.Y][pos.X+1:], b.cells[pos.Y][pos.X:])
	b.cells[pos.Y][pos.X] = term.Cell{Ch: r}
}

func (b *Cells) insertAt(pos term.Coordinates, r rune) (next term.Coordinates) {
	switch r {
	case '\n':
		b.insertNewRow(pos)
		next = term.Coordinates{X: 0, Y: pos.Y + 1}
	case '\t':
		if b.tabspaces > 0 {
			b.insertTabSpaces(pos)
			next = term.Coordinates{X: pos.X + b.tabspaces, Y: pos.Y}
			break
		}
		fallthrough
	default:
		b.doInsertAt(pos, r)
		next = term.Coordinates{X: pos.X + 1, Y: pos.Y}
	}

	return
}

func (b *Cells) insertTabSpaces(pos term.Coordinates) {
	n := term.Coordinates{X: pos.X, Y: pos.Y}
	for i := 1; i < b.tabspaces; i++ {
		n = b.insertAt(n, '\x00')
	}
	b.doInsertAt(n, '\t')
}

func (b *Cells) fillInRows(y int) (n int) {
	if b.cells == nil {
		b.Reset()
	}
	for y >= len(b.cells) {
		n++
		row := makeNewRow(0, defColumnCap)
		b.cells = append(b.cells, row)
	}
	return
}

func (b *Cells) fillInColumns(pos term.Coordinates) (n int) {
	for pos.X > len(b.cells[pos.Y]) {
		b.cells[pos.Y] = append(b.cells[pos.Y], term.Cell{Ch: ' '})
		n++
	}
	return
}

// Insert inserts string in the given position and shifts the remaining cells.
// Insert never fails: if at is out-of-bounds, this method fills in the rows
// and/or columns of cells.
func (b *Cells) Insert(at term.Coordinates, str string) (
	from, to term.Coordinates,
) {
	assertValidCoords(at)

	from = at
	if filled := b.fillInRows(at.Y); filled != 0 {
		from.Y -= filled
		from.X = b.Columns(from.Y)
		b.fillInColumns(at)
	} else {
		from.X -= b.fillInColumns(at)
	}

	to = at
	for _, r := range str {
		to = b.insertAt(to, r)
	}
	return
}

func writeToBuilder(builder *strings.Builder, cells [][]term.Cell) {
	for i, r := range cells {
		writeRowToBuilder(builder, r)
		if i+1 != len(cells) {
			builder.WriteByte('\n')
		}
	}
}

func writeRowToBuilder(builder *strings.Builder, cells []term.Cell) {
	for _, c := range cells {
		if c.Ch != '\x00' {
			builder.WriteRune(c.Ch)
		}
	}
}

func (b *Cells) canConflate(row int) (ok bool) {
	return row < len(b.cells)-1
}

func (b *Cells) conflate(row int) {
	// copy cells from next row into current row
	rlen := len(b.cells[row+1])
	if rlen != 0 {
		origLen := len(b.cells[row])
		b.cells[row] = append(b.cells[row], make([]term.Cell, rlen)...)
		copy(b.cells[row][origLen:], b.cells[row+1][:])
	}

	// copy all rows into row we just moved up and trim last row
	copy(b.cells[row+1:], b.cells[row+2:])
	b.cells = b.cells[:len(b.cells)-1]
}

func (b *Cells) skipPaddingLeft(row, x int) int {
	rowLastIdx := len(b.cells[row]) - 1
	for x < rowLastIdx && b.cells[row][x].Ch == 0 {
		if x == 0 {
			break
		}
		x--
	}
	return x
}

func (b *Cells) skipPaddingRight(row, x int) int {
	rowLastIdx := len(b.cells[row]) - 1
	for x < rowLastIdx && b.cells[row][x].Ch == 0 {
		if x == rowLastIdx {
			break
		}
		x++
	}
	return x
}

func (b *Cells) doDeleteRowInRange(
	builder *strings.Builder, row, fromX, toX int,
) (conflate bool) {
	fromX = b.skipPaddingLeft(row, fromX)
	toX = b.skipPaddingRight(row, toX)
	conflate = toX == len(b.cells[row])
	if !conflate {
		toX++
	}
	writeRowToBuilder(builder, b.cells[row][fromX:toX])
	diff := toX - fromX
	copy(b.cells[row][fromX:], b.cells[row][toX:])
	b.cells[row] = b.cells[row][:len(b.cells[row])-diff]

	return conflate
}

func (b *Cells) deleteRowInRange(
	builder *strings.Builder, row, fromX, toX int,
) {
	shouldConflate := b.doDeleteRowInRange(builder, row, fromX, toX)
	if shouldConflate && b.canConflate(row) {
		builder.WriteByte('\n')
		b.conflate(row)
	}
}

// Delete removes cells in left-inclusive, right-inclusive range
// and returns the corresponding string representation of the cells removed.
func (b *Cells) Delete(from, to term.Coordinates) string {
	from, to = sortFromTo(from, to)
	b.assertCordsInBounds(from)
	b.assertCordsInBounds(to)

	builder := strings.Builder{}

	if from.Y == to.Y {
		b.deleteRowInRange(&builder, from.Y, from.X, to.X)
		return builder.String()
	}

	// trim til end of first row
	b.doDeleteRowInRange(&builder, from.Y, from.X, len(b.cells[from.Y]))
	builder.WriteByte('\n')

	// copy second row until second to last, to reuse conflate logic
	if diff := to.Y - from.Y; diff > 1 {
		writeToBuilder(&builder, b.cells[from.Y+1:to.Y])
		builder.WriteByte('\n')

		b.cells[from.Y+1] = b.cells[to.Y]
		to.Y = from.Y + 1
		b.cells = b.cells[:len(b.cells)-diff+1]
	}

	// then remove cells from last row; from.Y is now last row to delete
	b.deleteRowInRange(&builder, to.Y, 0, to.X)

	// conflate last row in range
	b.conflate(from.Y)

	return builder.String()
}

// Columns returns the number of cells of row at index y
func (b *Cells) Columns(y int) (j int) {
	if b.cells == nil {
		b.Reset()
	}
	if y < 0 {
		panic(fmt.Sprintf("invalid row: %d", y))
	}
	if y >= b.Rows() {
		panic(fmt.Sprintf("row out of bounds: %d", y))
	}
	j = len(b.cells[y])
	return
}

// Rows returns the number of rows in the buffer
func (b *Cells) Rows() int {
	return len(b.cells)
}

func (b *Cells) String() string {
	builder := strings.Builder{}
	writeToBuilder(&builder, b.cells)
	return builder.String()
}

// RawCells gives clients access to the underlying cell matrix.
func (b *Cells) RawCells() [][]term.Cell {
	if b.cells == nil {
		b.Reset()
	}
	return b.cells
}

// Cell returns the cell and true or a zero-valued cell and false if there is no
// cell at position. If attempting to get a tab padding, the position of the tab is returned.
func (b *Cells) Cell(pos term.Coordinates) (
	ppos term.Coordinates, cell term.Cell,
) {
	b.assertCordsInBounds(pos)

	if pos.X == len(b.cells[pos.Y]) {
		cell.Ch = '\n'
		ppos = pos
		return
	}

	for cell.Ch == 0 {
		cell = b.cells[pos.Y][pos.X]
		pos.X++
	}
	ppos = pos
	return
}

// ReadFrom reads data from r until EOF and appends it to the buffer, growing
// the buffer as needed. The return value n is the number of bytes read. Any
// error except io.EOF encountered during the read is also returned.
func (b *Cells) ReadFrom(r io.Reader) (n int64, err error) {
	if b.cells == nil {
		b.Reset()
	}
	rowY := b.NextWrite().Y
	reader := bufio.NewReader(r)
	// TODO fix case when rune length is > 1 without sacrificing perf too much
	var bytes []byte
	var isPrefix bool
	for {
		bytes, isPrefix, err = reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				if n > 0 {
					n--
					b.cells = b.cells[:len(b.cells)-1]
				}
				err = nil
			}
			return
		}
		for _, r := range bytes {
			for i := 1; r == '\t' && i < b.tabspaces; i++ {
				b.cells[rowY] = append(b.cells[rowY], term.Cell{})
			}
			b.cells[rowY] = append(b.cells[rowY], term.Cell{Ch: rune(r)})
		}
		n += int64(len(bytes))
		if !isPrefix {
			b.cells = append(b.cells, makeNewRow(0, defColumnCap))
			rowY++
			n++
		}
	}
}

// Tabspaces returns the number of tabspaces initialized.
func (b *Cells) Tabspaces() int {
	return b.tabspaces
}

// NextWrite returns the position of the write cursor.
func (b *Cells) NextWrite() term.Coordinates {
	if b.cells == nil {
		b.Reset()
	}
	// cannot be 0, since we always have at least one row
	y := len(b.cells) - 1
	x := len(b.cells[y])
	return term.Coordinates{X: x, Y: y}
}
