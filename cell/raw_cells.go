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
	cells     [][]term.Cell
	tabspaces int
}

// Init initializes this RawCells with the given tabspaces config and resets its contents.
func (c *RawCells) Init(tabspaces int) {
	c.tabspaces = tabspaces
	c.Reset()
}

// Reset resets the contents of this cellbuf.
func (c *RawCells) Reset() {
	c.cells = make([][]term.Cell, 1, defRowCap)
	c.cells[0] = makeNewRow(0, defColumnCap)
	if c.tabspaces == 0 {
		c.tabspaces = defTabSpaces
	}
}

func assertValidCoords(pos term.Coordinates) {
	if pos.X < 0 || pos.Y < 0 {
		panic(fmt.Sprintf("invalid coordinates: %+v", pos))
	}
}

func (c *RawCells) assertCordsInBounds(pos term.Coordinates) {
	assertValidCoords(pos)
	if pos.Y >= c.Rows() || pos.X > len(c.cells[pos.Y]) {
		panic(fmt.Sprintf("Coordinates out of bounds: %+v", pos))
	}
}

func makeNewRow(length, capacity int) (row []term.Cell) {
	row = make([]term.Cell, length, capacity)
	return
}

func (c *RawCells) insertNewRow(pos term.Coordinates) {
	assertValidCoords(pos)
	sourceRow := c.cells[pos.Y]
	targetY := pos.Y + 1

	// make enough space for one more row
	c.cells = append(c.cells, nil)
	copy(c.cells[targetY:], c.cells[pos.Y:])

	// if not last position, copy the rest of cells to the next row
	if pos.X < len(sourceRow) {
		c.cells[pos.Y] = c.cells[pos.Y][:pos.X]
		length := len(sourceRow[pos.X:])
		c.cells[targetY] = makeNewRow(length, length)
		copy(c.cells[targetY], sourceRow[pos.X:])
	} else {
		c.cells[targetY] = makeNewRow(0, defColumnCap)
	}
}

func (c *RawCells) doInsertAt(pos term.Coordinates, r rune) {
	// make sure we have enough capacity
	c.cells[pos.Y] = append(c.cells[pos.Y], term.Cell{})
	copy(c.cells[pos.Y][pos.X+1:], c.cells[pos.Y][pos.X:])
	c.cells[pos.Y][pos.X] = term.Cell{Ch: r}
}

func (c *RawCells) insertAt(pos term.Coordinates, r rune) (next term.Coordinates) {
	switch r {
	case '\n':
		c.insertNewRow(pos)
		next = term.Coordinates{X: 0, Y: pos.Y + 1}
	case '\t':
		if c.tabspaces > 0 {
			c.insertTabSpaces(pos)
			next = term.Coordinates{X: pos.X + c.tabspaces, Y: pos.Y}
			break
		}
		fallthrough
	default:
		c.doInsertAt(pos, r)
		next = term.Coordinates{X: pos.X + 1, Y: pos.Y}
	}

	return
}

func (c *RawCells) insertTabSpaces(pos term.Coordinates) {
	n := term.Coordinates{X: pos.X, Y: pos.Y}
	for i := 1; i < c.tabspaces; i++ {
		n = c.insertAt(n, '\x00')
	}
	c.doInsertAt(n, '\t')
}

func (c *RawCells) fillInRows(y int) (n int) {
	if c.cells == nil {
		c.Reset()
	}
	for y >= len(c.cells) {
		n++
		row := makeNewRow(0, defColumnCap)
		c.cells = append(c.cells, row)
	}
	return
}

func (c *RawCells) fillInColumns(pos term.Coordinates) (n int) {
	for pos.X > len(c.cells[pos.Y]) {
		c.cells[pos.Y] = append(c.cells[pos.Y], term.Cell{Ch: ' '})
		n++
	}
	return
}

func (c *RawCells) fillInCoords(pos term.Coordinates) (
	from, to term.Coordinates,
) {
	assertValidCoords(pos)
	from = pos
	if filled := c.fillInRows(pos.Y); filled != 0 {
		from.Y -= filled
		from.X = c.Columns(from.Y)
		c.fillInColumns(pos)
	} else {
		from.X -= c.fillInColumns(pos)
	}
	to = pos

	return
}

// Insert inserts string in the given position and shifts the remaining cells.
// Insert never fails: if at is out-of-bounds, this method fills in the rows
// and/or columns of cells.
func (c *RawCells) Insert(at term.Coordinates, str string) (
	from, to term.Coordinates,
) {
	from, to = c.fillInCoords(at)
	next := to
	for _, r := range str {
		to = next
		next = c.insertAt(next, r)
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

func (c *RawCells) canConflate(row int) (ok bool) {
	return row < len(c.cells)-1
}

func (c *RawCells) conflate(row int) {
	// copy cells from next row into current row
	rlen := len(c.cells[row+1])
	if rlen != 0 {
		origLen := len(c.cells[row])
		c.cells[row] = append(c.cells[row], make([]term.Cell, rlen)...)
		copy(c.cells[row][origLen:], c.cells[row+1][:])
	}

	// copy all rows into row we just moved up and trim last row
	copy(c.cells[row+1:], c.cells[row+2:])
	c.cells = c.cells[:len(c.cells)-1]
}

func (c *RawCells) doDeleteRowInRange(
	builder *strings.Builder, row, fromX, toX int,
) (conflate bool) {
	conflate = toX == len(c.cells[row])
	if !conflate {
		toX++
	}
	copyRowToBuilder(builder, c.cells[row][fromX:toX])
	diff := toX - fromX
	copy(c.cells[row][fromX:], c.cells[row][toX:])
	c.cells[row] = c.cells[row][:len(c.cells[row])-diff]

	return conflate
}

func (c *RawCells) deleteRowRange(
	builder *strings.Builder, row, fromX, toX int,
) {
	shouldConflate := c.doDeleteRowInRange(builder, row, fromX, toX)
	if shouldConflate && c.canConflate(row) {
		builder.WriteByte('\n')
		c.conflate(row)
	}
}

func (c *RawCells) skipPadding(start, end term.Coordinates) (
	term.Coordinates, term.Coordinates,
) {
	tokens := c.tabspaces - 1
	rowLastIdx := len(c.cells[end.Y]) - 1
	for tokens > 0 && end.X < rowLastIdx && c.cells[end.Y][end.X].Ch == 0 {
		end.X++
		tokens--
	}

	tokens = c.tabspaces - 1
	rowLastIdx = len(c.cells[start.Y]) - 1
	for tokens > 0 && start.X > 0 &&
		start.X < rowLastIdx && c.cells[start.Y][start.X].Ch == 0 {

		start.X--
		tokens--

	}

	// we need the last pad's position, but on the left there's no \t delimiter,
	// so we need to rollback one cell
	if tokens != c.tabspaces-1 && c.cells[start.Y][start.X].Ch != 0 {
		start.X++
	}
	return start, end
}

// Delete removes cells in left-inclusive, right-inclusive range
// and returns the corresponding string representation of the cells removed,
// along with the true start and end of the range, in case some cells groups
// (cell with padding) were deleted.
func (c *RawCells) Delete(from, to term.Coordinates) (
	start, end term.Coordinates, str string,
) {
	c.assertCordsInBounds(start)
	c.assertCordsInBounds(end)
	start, end = sortFromTo(from, to)
	start, end = c.skipPadding(start, end)

	builder := strings.Builder{}

	if start.Y == end.Y {
		c.deleteRowRange(&builder, start.Y, start.X, end.X)
		str = builder.String()
		return
	}

	// trim til end of first row
	c.doDeleteRowInRange(&builder, start.Y, start.X, len(c.cells[start.Y]))
	builder.WriteByte('\n')

	// copy rows in between and move last row to second row, if applicable
	lastRow := end.Y
	if diff := lastRow - start.Y; diff > 1 {
		copyToBuilder(&builder, c.cells[start.Y+1:lastRow])
		builder.WriteByte('\n')

		c.cells[start.Y+1] = c.cells[lastRow]
		lastRow = start.Y + 1
		c.cells = c.cells[:len(c.cells)-diff+1]
	}

	// then remove cells from last row; start.Y is now last row to delete
	c.deleteRowRange(&builder, lastRow, 0, end.X)

	// conflate last row in range
	c.conflate(start.Y)

	str = builder.String()
	return
}

// Columns returns the number of cells of row at index y
func (c *RawCells) Columns(y int) (j int) {
	if c.cells == nil {
		c.Reset()
	}
	if y < 0 {
		panic(fmt.Sprintf("invalid row: %d", y))
	}
	if y >= c.Rows() {
		panic(fmt.Sprintf("row out of bounds: %d", y))
	}
	j = len(c.cells[y])
	return
}

// Rows returns the number of rows in the buffer
func (c *RawCells) Rows() int {
	return len(c.cells)
}

func (c *RawCells) String() string {
	return CellsToString(c.cells)
}

// CellsToString returns the string representation of the given cell matrix.
func CellsToString(cells [][]term.Cell) string {
	builder := strings.Builder{}
	copyToBuilder(&builder, cells)
	return builder.String()
}

// RawCells gives clients access to the underlying cell matrix.
func (c *RawCells) RawCells() [][]term.Cell {
	if c.cells == nil {
		c.Reset()
	}
	return c.cells
}

// Cell returns the cell and true or a zero-valued cell and false if there is no
// cell at position. If attempting to get a tab padding, the position of the tab is returned.
func (c *RawCells) Cell(pos term.Coordinates) (
	ppos term.Coordinates, cell term.Cell,
) {
	c.assertCordsInBounds(pos)

	if pos.X == len(c.cells[pos.Y]) {
		cell.Ch = '\n'
		ppos = pos
		return
	}

	for cell.Ch == 0 {
		cell = c.cells[pos.Y][pos.X]
		pos.X++
	}
	ppos = pos
	return
}

// ReadFrom reads data from r until EOF and appends it to the buffer, growing
// the buffer as needed. The return value n is the number of bytes read. Any
// error except io.EOF encountered during the read is also returned.
func (c *RawCells) ReadFrom(r io.Reader) (int64, error) {
	if c.cells == nil {
		c.Reset()
	}
	rowY := c.NextWrite().Y
	reader := bufio.NewReader(r)
	n := int64(0)
	for {
		str, err := reader.ReadString('\n')
		for _, r := range str {
			switch r {
			case '\n':
			case '\t':
				for i := 1; r == '\t' && i < c.tabspaces; i++ {
					c.cells[rowY] = append(c.cells[rowY], term.Cell{})
				}
				fallthrough
			default:
				c.cells[rowY] = append(c.cells[rowY], term.Cell{Ch: rune(r)})
			}
		}
		n += int64(len(str))
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return n, err
		}
		c.cells = append(c.cells, makeNewRow(0, defColumnCap))
		rowY++
	}
}

// NextWrite returns the position of the write cursor.
func (c *RawCells) NextWrite() term.Coordinates {
	if c.cells == nil {
		c.Reset()
	}
	// cannot be 0, since we always have at least one row
	y := len(c.cells) - 1
	x := len(c.cells[y])
	return term.Coordinates{X: x, Y: y}
}
