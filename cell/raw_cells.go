package cell

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/ernestrc/go-tui/term"
)

const defTabSpaces int = 4
const defColumnCap int = 64
const defRowCap int = 64

// rawCells is a matrix of term.Cell. The zero value for rawCells is ready to use.
type rawCells struct {
	cells     [][]term.Cell
	tabspaces int
}

// init initializes this rawCells with the given tabspaces config and resets its contents.
func (c *rawCells) init(tabspaces int) {
	c.tabspaces = tabspaces
	c.reset()
}

func (c *rawCells) reset() {
	c.cells = make([][]term.Cell, 1, defRowCap)
	c.cells[0] = makeNewRow(0, defColumnCap)
}

func assertValidCoords(pos term.Coordinates) {
	if pos.X < 0 || pos.Y < 0 {
		panic(fmt.Sprintf("invalid coordinates: %+v", pos))
	}
}

func (c *rawCells) assertCordsInBounds(pos term.Coordinates) {
	assertValidCoords(pos)
	if pos.Y >= c.Rows() || pos.X > len(c.cells[pos.Y]) {
		panic(fmt.Sprintf("Coordinates out of bounds: %+v", pos))
	}
}

func makeNewRow(length, capacity int) (row []term.Cell) {
	row = make([]term.Cell, length, capacity)
	return
}

func (c *rawCells) insertNewRow(pos term.Coordinates) {
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

func (c *rawCells) doInsertAt(pos term.Coordinates, r rune) {
	// make sure we have enough capacity
	c.cells[pos.Y] = append(c.cells[pos.Y], term.Cell{})
	copy(c.cells[pos.Y][pos.X+1:], c.cells[pos.Y][pos.X:])
	c.cells[pos.Y][pos.X] = term.Cell{Ch: r}
}

func (c *rawCells) insertAt(pos term.Coordinates, r rune) (next term.Coordinates) {
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

func (c *rawCells) insertTabSpaces(pos term.Coordinates) {
	n := term.Coordinates{X: pos.X, Y: pos.Y}
	for i := 1; i < c.tabspaces; i++ {
		n = c.insertAt(n, '\x00')
	}
	c.doInsertAt(n, '\t')
}

func (c *rawCells) fillInRows(y int) (n int) {
	for y >= len(c.cells) {
		n++
		row := makeNewRow(0, defColumnCap)
		c.cells = append(c.cells, row)
	}
	return
}

func (c *rawCells) fillInColumns(pos term.Coordinates) (n int) {
	for pos.X > len(c.cells[pos.Y]) {
		c.cells[pos.Y] = append(c.cells[pos.Y], term.Cell{Ch: ' '})
		n++
	}
	return
}

func (c *rawCells) fillInCoords(pos term.Coordinates) (
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

func (c *rawCells) Insert(at term.Coordinates, str string) (
	from, to term.Coordinates,
) {
	from, to = c.fillInCoords(at)
	// avoid breaking padding blocks in half
	if from == at {
		from, _ = c.skipPadding(at, at)
		to = from
	}
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

func (c *rawCells) canConflate(row int) (ok bool) {
	return row < len(c.cells)-1
}

func (c *rawCells) conflate(row int) {
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

func (c *rawCells) doDeleteRowInRange(
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

// returns true if should have conflated but could not;
// that's useful to correct end coordinate when trying to delete
// last row til last newline
func (c *rawCells) deleteRowRange(
	builder *strings.Builder, row, fromX, toX int,
) bool {
	shouldConflate := c.doDeleteRowInRange(builder, row, fromX, toX)
	canConflate := c.canConflate(row)
	if shouldConflate && canConflate {
		builder.WriteByte('\n')
		c.conflate(row)
		return false
	}
	return shouldConflate && !canConflate
}

func (c *rawCells) skipPadding(start, end term.Coordinates) (
	term.Coordinates, term.Coordinates,
) {
	tokens := c.tabspaces - 1
	endLastIdx := len(c.cells[end.Y]) - 1
	startLastIdx := len(c.cells[start.Y]) - 1

	for tokens > 0 && end.X < endLastIdx && c.cells[end.Y][end.X].Ch == 0 {
		end.X++
		tokens--
	}

	if start.X > 0 && start.X <= startLastIdx &&
		c.cells[start.Y][start.X].Ch == '\t' {
		start.X--
	}

	tokens = c.tabspaces - 1
	for tokens > 0 && start.X > 0 &&
		start.X <= startLastIdx && c.cells[start.Y][start.X].Ch == 0 {
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

func (c *rawCells) Delete(from, to term.Coordinates) (
	start, end term.Coordinates, str string,
) {
	c.assertCordsInBounds(from)
	c.assertCordsInBounds(to)
	start, end = SortFromTo(from, to)
	start, end = c.skipPadding(start, end)

	builder := strings.Builder{}

	if start.Y == end.Y {
		if c.deleteRowRange(&builder, start.Y, start.X, end.X) {
			end.X--
		}
		str = builder.String()
		return
	}

	// trim til end of first row
	c.doDeleteRowInRange(&builder, start.Y, start.X, len(c.cells[start.Y]))
	builder.WriteByte('\n')

	// copy rows in between and move last row to second row, if applicable
	lastRow := end.Y
	if diff := end.Y - start.Y; diff > 1 {
		copyToBuilder(&builder, c.cells[start.Y+1:end.Y])
		builder.WriteByte('\n')

		copy(c.cells[start.Y+1:], c.cells[end.Y:])
		c.cells = c.cells[:len(c.cells)-diff+1]

		lastRow = start.Y + 1
	}

	// then remove cells from last row; start.Y is now last row to delete
	if c.deleteRowRange(&builder, lastRow, 0, end.X) {
		end.X--
	}

	// conflate last row in range
	c.conflate(start.Y)

	str = builder.String()

	return
}

func (c *rawCells) Columns(y int) (j int) {
	j = len(c.cells[y])
	return
}

func (c *rawCells) Rows() int {
	return len(c.cells)
}

func (c *rawCells) String() string {
	return CellsToString(c.RawCells())
}

func (c *rawCells) RawCells() [][]term.Cell {
	return c.cells
}

func (c *rawCells) Cell(pos term.Coordinates) (
	cell term.Cell, ok bool,
) {
	assertValidCoords(pos)
	if pos.Y >= c.Rows() || pos.X >= len(c.cells[pos.Y]) {
		return
	}
	cell = c.cells[pos.Y][pos.X]
	ok = true
	return
}

func (c *rawCells) ReadFrom(r io.Reader) (int64, error) {
	rowY := c.nextWrite().Y
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

// nextWrite returns the position of the write cursor.
func (c *rawCells) nextWrite() term.Coordinates {
	y := c.Rows() - 1
	x := len(c.cells[y])
	return term.Coordinates{X: x, Y: y}
}
