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

// RawCells is a matrix of term.Cell. The zero value for RawCells is ready to use.
// It satisfies cell.Reader and cell.Writer.
type RawCells struct {
	// TODO should be a matrix of rune; to solve scroll search:
	// - scroll search results should be recalculated on every draw?
	// - should be a matrix of a struct { rune and a ctx } field which could be
	//   an interface type so it could be dynamic or simply a context.Context?
	cells     [][]term.Cell
	tabspaces int
}

// Init initializes this RawCells with the given tabspaces config and resets its contents.
func (b *RawCells) Init(tabspaces int) {
	b.tabspaces = tabspaces
	b.Reset()
}

// Reset resets the contents of this cellbuf.
func (b *RawCells) Reset() {
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

func (b *RawCells) assertCordsInBounds(pos term.Coordinates) {
	assertValidCoords(pos)
	if pos.Y >= b.Rows() || pos.X > len(b.cells[pos.Y]) {
		panic(fmt.Sprintf("Coordinates out of bounds: %+v", pos))
	}
}

func makeNewRow(length, capacity int) (row []term.Cell) {
	row = make([]term.Cell, length, capacity)
	return
}

func (b *RawCells) insertNewRow(pos term.Coordinates) {
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

func (b *RawCells) doInsertAt(pos term.Coordinates, r rune) {
	// make sure we have enough capacity
	b.cells[pos.Y] = append(b.cells[pos.Y], term.Cell{})
	copy(b.cells[pos.Y][pos.X+1:], b.cells[pos.Y][pos.X:])
	b.cells[pos.Y][pos.X] = term.Cell{Ch: r}
}

func (b *RawCells) insertAt(pos term.Coordinates, r rune) (next term.Coordinates) {
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

func (b *RawCells) insertTabSpaces(pos term.Coordinates) {
	n := term.Coordinates{X: pos.X, Y: pos.Y}
	for i := 1; i < b.tabspaces; i++ {
		n = b.insertAt(n, '\x00')
	}
	b.doInsertAt(n, '\t')
}

func (b *RawCells) fillInRows(y int) (n int) {
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

func (b *RawCells) fillInColumns(pos term.Coordinates) (n int) {
	for pos.X > len(b.cells[pos.Y]) {
		b.cells[pos.Y] = append(b.cells[pos.Y], term.Cell{Ch: ' '})
		n++
	}
	return
}

func (b *RawCells) fillInCoords(pos term.Coordinates) (
	from, to term.Coordinates,
) {
	assertValidCoords(pos)
	from = pos
	if filled := b.fillInRows(pos.Y); filled != 0 {
		from.Y -= filled
		from.X = b.Columns(from.Y)
		b.fillInColumns(pos)
	} else {
		from.X -= b.fillInColumns(pos)
	}
	to = pos

	return
}

// Insert inserts string in the given position and shifts the remaining cells.
// Insert never fails: if at is out-of-bounds, this method fills in the rows
// and/or columns of cells.
func (b *RawCells) Insert(at term.Coordinates, str string) (
	from, to term.Coordinates,
) {
	from, to = b.fillInCoords(at)
	next := to
	for _, r := range str {
		to = next
		next = b.insertAt(next, r)
		if padding := next.X - to.X - 1; padding > 0 {
			to.X += padding
		}
	}
	return
}

func copyToBuilder(builder *strings.Builder, cells [][]term.Cell) {
	for i, r := range cells {
		copyRowToBuilder(builder, r)
		if i+1 != len(cells) {
			builder.WriteByte('\n')
		}
	}
}

func copyRowToBuilder(builder *strings.Builder, cells []term.Cell) {
	for _, c := range cells {
		if c.Ch != '\x00' {
			builder.WriteRune(c.Ch)
		}
	}
}

func (b *RawCells) canConflate(row int) (ok bool) {
	return row < len(b.cells)-1
}

func (b *RawCells) conflate(row int) {
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

func (b *RawCells) doDeleteRowInRange(
	builder *strings.Builder, row, fromX, toX int,
) (conflate bool) {
	conflate = toX == len(b.cells[row])
	if !conflate {
		toX++
	}
	copyRowToBuilder(builder, b.cells[row][fromX:toX])
	diff := toX - fromX
	copy(b.cells[row][fromX:], b.cells[row][toX:])
	b.cells[row] = b.cells[row][:len(b.cells[row])-diff]

	return conflate
}

func (b *RawCells) deleteRowRange(
	builder *strings.Builder, row, fromX, toX int,
) {
	shouldConflate := b.doDeleteRowInRange(builder, row, fromX, toX)
	if shouldConflate && b.canConflate(row) {
		builder.WriteByte('\n')
		b.conflate(row)
	}
}

func (b *RawCells) skipPadding(start, end term.Coordinates) (
	term.Coordinates, term.Coordinates,
) {
	tokens := b.tabspaces - 1
	rowLastIdx := len(b.cells[end.Y]) - 1
	for tokens > 0 && end.X < rowLastIdx && b.cells[end.Y][end.X].Ch == 0 {
		end.X++
		tokens--
	}

	tokens = b.tabspaces - 1
	rowLastIdx = len(b.cells[start.Y]) - 1
	for tokens > 0 && start.X > 0 &&
		start.X < rowLastIdx && b.cells[start.Y][start.X].Ch == 0 {

		start.X--
		tokens--

	}

	// we need the last pad's position, but on the left there's no \t delimiter,
	// so we need to rollback one cell
	if tokens != b.tabspaces-1 && b.cells[start.Y][start.X].Ch != 0 {
		start.X++
	}
	return start, end
}

// Delete removes cells in left-inclusive, right-inclusive range
// and returns the corresponding string representation of the cells removed,
// along with the true start and end of the range, in case some cells groups
// (cell with padding) were deleted.
func (b *RawCells) Delete(from, to term.Coordinates) (
	start, end term.Coordinates, str string,
) {
	b.assertCordsInBounds(start)
	b.assertCordsInBounds(end)
	start, end = sortFromTo(from, to)
	start, end = b.skipPadding(start, end)

	builder := strings.Builder{}

	if start.Y == end.Y {
		b.deleteRowRange(&builder, start.Y, start.X, end.X)
		str = builder.String()
		return
	}

	// trim til end of first row
	b.doDeleteRowInRange(&builder, start.Y, start.X, len(b.cells[start.Y]))
	builder.WriteByte('\n')

	// copy rows in between and move last row to second row, if applicable
	lastRow := end.Y
	if diff := lastRow - start.Y; diff > 1 {
		copyToBuilder(&builder, b.cells[start.Y+1:lastRow])
		builder.WriteByte('\n')

		b.cells[start.Y+1] = b.cells[lastRow]
		lastRow = start.Y + 1
		b.cells = b.cells[:len(b.cells)-diff+1]
	}

	// then remove cells from last row; start.Y is now last row to delete
	b.deleteRowRange(&builder, lastRow, 0, end.X)

	// conflate last row in range
	b.conflate(start.Y)

	str = builder.String()
	return
}

// Columns returns the number of cells of row at index y
func (b *RawCells) Columns(y int) (j int) {
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
func (b *RawCells) Rows() int {
	return len(b.cells)
}

func (b *RawCells) String() string {
	builder := strings.Builder{}
	copyToBuilder(&builder, b.cells)
	return builder.String()
}

// RawCells gives clients access to the underlying cell matrix.
func (b *RawCells) RawCells() [][]term.Cell {
	if b.cells == nil {
		b.Reset()
	}
	return b.cells
}

// Cell returns the cell and true or a zero-valued cell and false if there is no
// cell at position. If attempting to get a tab padding, the position of the tab is returned.
func (b *RawCells) Cell(pos term.Coordinates) (
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
func (b *RawCells) ReadFrom(r io.Reader) (int64, error) {
	if b.cells == nil {
		b.Reset()
	}
	rowY := b.NextWrite().Y
	reader := bufio.NewReader(r)
	n := int64(0)
	for {
		str, err := reader.ReadString('\n')
		for _, r := range str {
			switch r {
			case '\n':
			case '\t':
				for i := 1; r == '\t' && i < b.tabspaces; i++ {
					b.cells[rowY] = append(b.cells[rowY], term.Cell{})
				}
				fallthrough
			default:
				b.cells[rowY] = append(b.cells[rowY], term.Cell{Ch: rune(r)})
			}
		}
		n += int64(len(str))
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return n, err
		}
		b.cells = append(b.cells, makeNewRow(0, defColumnCap))
		rowY++
	}
}

// NextWrite returns the position of the write cursor.
func (b *RawCells) NextWrite() term.Coordinates {
	if b.cells == nil {
		b.Reset()
	}
	// cannot be 0, since we always have at least one row
	y := len(b.cells) - 1
	x := len(b.cells[y])
	return term.Coordinates{X: x, Y: y}
}
