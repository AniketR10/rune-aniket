// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package cell

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/term/graphemecluster"
)

const (
	defColumnCap int = 64
	defRowCap    int = 64
)

// rawCells is a matrix of term.Cell.
type rawCells struct {
	columnCap  int
	rowCap     int
	cells      [][]term.Cell
	fillInChar rune
	zwj        bool
	zwjPos     term.Coordinates
}

// init initializes this rawCells with the given tabspaces config and resets its contents.
func (c *rawCells) init() {
	c.fillInChar = ' '
	c.reset()
}

func (c *rawCells) initWithCap(rowCap, columnCap int, fillInChar rune) {
	c.fillInChar = fillInChar
	c.resetWithCap(rowCap, columnCap)
}

func (c *rawCells) reset() {
	c.resetWithCap(defRowCap, defColumnCap)
}

func (c *rawCells) resetWithCap(rowCap, columnCap int) {
	c.columnCap = int(math.Max(float64(columnCap), float64(defColumnCap)))
	c.rowCap = int(math.Max(float64(rowCap), float64(defRowCap)))
	c.cells = make([][]term.Cell, 1, c.rowCap)
	c.cells[0] = makeNewRow(0, c.columnCap)
	c.zwj = false
	c.zwjPos = term.Coordinates{}
}

// adoptCells discards this rawCells' contents and adopts cells in place.
// The *rawCells identity (and therefore any Editor/View pinned to it via
// cell.Buffer) is preserved. Intended for bulk reloads (e.g. snapshot
// restore) that must not be observable as an Edit.
//
// columnCap/rowCap/fillInChar are left intact — they're configuration,
// not content. The cells slice is adopted directly; callers must clone
// when the source slice is shared.
//
// Panics if cells is empty: rawCells maintains the invariant that there
// is always at least one row, and silently substituting a fresh row
// would break the caller's expectation that the passed slice is the
// authoritative content.
func (c *rawCells) adoptCells(cells [][]term.Cell) {
	if len(cells) == 0 {
		panic("rawCells.adoptCells: cells must contain at least one row")
	}
	c.cells = cells
	c.zwj = false
	c.zwjPos = term.Coordinates{}
}

func assertValidCoords(pos term.Coordinates) {
	if pos.X < 0 || pos.Y < 0 {
		panic(fmt.Sprintf("invalid coordinates: %+v", pos))
	}
}

func makeNewRow(length, capacity int) (row []term.Cell) {
	capacity = int(math.Max(float64(length), float64(capacity)))
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
		c.cells[targetY] = makeNewRow(length, c.columnCap)
		copy(c.cells[targetY], sourceRow[pos.X:])
	} else {
		c.cells[targetY] = makeNewRow(0, c.columnCap)
	}
}

// splitRowsBatch applies a batch of splits to rows in c.cells with a single
// outer-slice allocation (at most). Splits must reference distinct, strictly
// increasing row indices.
//
// Each split at row Y with (Width, Times) re-slices c.cells[Y] into
// (Times+1) pieces of length Width (head and intermediate pieces) and a
// trailing piece containing whatever remained. The original row slice is
// reused with 3-index slicing (cap = len), so sub-pieces do not share
// append-writable capacity with their neighbours.
//
// If padToWidth > 0 and a tail piece ends up shorter than padToWidth, the
// tail is materialized as a fresh slice of length padToWidth with the
// original cells copied into the first positions and term.Cell{Ch: fillChar}
// filling the rest. All such tail materializations share a single slab
// allocation so per-tail allocations are avoided.
//
// Returns the total number of rows added.
func (c *rawCells) splitRowsBatch(splits []RowSplit, padToWidth int, fillChar rune) (added int) {
	if len(splits) == 0 {
		return 0
	}
	for _, s := range splits {
		if s.Times <= 0 || s.Width <= 0 {
			continue
		}
		added += s.Times
	}
	if added == 0 {
		return 0
	}

	oldLen := len(c.cells)
	newLen := oldLen + added

	var dst [][]term.Cell
	if cap(c.cells) >= newLen {
		dst = c.cells[:newLen]
	} else {
		dst = make([][]term.Cell, newLen, newLen+newLen/2)
	}

	// If padding tails, pre-allocate one slab large enough to hold every
	// tail that needs padding. We only fill the cells that won't be copied
	// over from the original tail.
	var padSlab []term.Cell
	var padOff int
	var fill term.Cell
	if padToWidth > 0 {
		var padTails int
		for _, s := range splits {
			if s.Times <= 0 || s.Width <= 0 {
				continue
			}
			orig := c.cells[s.Y]
			tailLen := len(orig) - s.Times*s.Width
			if tailLen < padToWidth {
				padTails++
			}
		}
		if padTails > 0 {
			padSlab = make([]term.Cell, padTails*padToWidth)
			fill = term.Cell{Ch: fillChar}
		}
	}

	// Walk backwards over the original rows. Maintain a `shift` that tracks
	// how many positions everything below the current read pointer has been
	// pushed down by accumulated splits (from later, higher-index, original
	// rows). Each split inserts s.Times rows into the OUTPUT, after its head.
	//
	// We process splits from last to first. Between splits, we copy the
	// untouched tail rows down by `shift` positions.
	splitIdx := len(splits) - 1
	read := oldLen
	write := newLen
	for read > 0 {
		// Find next applicable split with Y < read.
		for splitIdx >= 0 {
			s := splits[splitIdx]
			if s.Times <= 0 || s.Width <= 0 {
				splitIdx--
				continue
			}
			if s.Y < read {
				break
			}
			splitIdx--
		}
		var nextSplitY int
		if splitIdx >= 0 {
			nextSplitY = splits[splitIdx].Y
		} else {
			nextSplitY = -1
		}

		// Copy untouched rows in (nextSplitY, read) down to dst.
		untouched := read - (nextSplitY + 1)
		if untouched > 0 {
			copy(dst[write-untouched:write], c.cells[nextSplitY+1:read])
			write -= untouched
		}

		if splitIdx < 0 {
			break
		}
		// Apply the split at splits[splitIdx].
		s := splits[splitIdx]
		orig := c.cells[s.Y]
		origLen := len(orig)
		// Emit the trailing pieces (j = s.Times .. 1), then the head at s.Y.
		for j := s.Times; j >= 1; j-- {
			start := j * s.Width
			var end int
			if j == s.Times {
				end = origLen
			} else {
				end = start + s.Width
			}
			write--
			if j == s.Times && padToWidth > 0 && end-start < padToWidth && padSlab != nil {
				// Materialize tail into padSlab at full padToWidth. Only
				// the pad region (past the original tail length) needs
				// fill; the original tail cells are copied in.
				slot := padSlab[padOff : padOff+padToWidth : padOff+padToWidth]
				tailLen := end - start
				copy(slot[:tailLen], orig[start:end])
				for i := tailLen; i < padToWidth; i++ {
					slot[i] = fill
				}
				dst[write] = slot
				padOff += padToWidth
			} else {
				dst[write] = orig[start:end:end]
			}
		}
		write--
		dst[write] = orig[:s.Width:s.Width]
		read = s.Y
		splitIdx--
	}

	c.cells = dst
	return added
}

func (c *rawCells) doInsertAt(pos term.Coordinates, r []rune, width, byteCount uint8) {
	// make sure we have enough capacity
	c.cells[pos.Y] = append(c.cells[pos.Y], term.Cell{})
	copy(c.cells[pos.Y][pos.X+1:], c.cells[pos.Y][pos.X:])
	var combining []rune
	if len(r) > 1 {
		combining = r[1:]
	}
	cell := term.Cell{Ch: r[0], Width: width, Bytes: byteCount}
	cell.SetCombining(combining)
	c.cells[pos.Y][pos.X] = cell
}

func (c *rawCells) insertAt(pos term.Coordinates, r []rune, width uint8, byteCount uint8) (
	next term.Coordinates,
) {
	switch r[0] {
	// zero-width joiner, at position 0, indicates that previous cell is not complete
	// This mechanism is needed because input event processes one rune at a time
	// This assumes that a zwj and the runes of the grapheme cluster it belongs to
	// are inserted sequentially
	case '\u200d':
		c.zwj = true
		c.doInsertAt(pos, r, width, byteCount)
		next = term.Coordinates{X: pos.X + 1, Y: pos.Y}
		c.zwjPos = next
		return
	case '\n':
		c.insertNewRow(pos)
		next = term.Coordinates{X: 0, Y: pos.Y + 1}
	default:
		at := pos
		c.doInsertAt(at, r, width, byteCount)
		next = term.Coordinates{X: at.X + 1, Y: at.Y}
	}

	if c.zwj {
		if c.zwjPos == pos {
			str := c.String()
			c.reset()
			_, _ = c.readFromWithView(strings.NewReader(str), c)
			next = pos
			next.X-- // cells were combined and \u200d removed
		}
		c.zwj = false
		c.zwjPos = term.Coordinates{}
	}

	return
}

// specialized, most common case for perf improvement
func (c *rawCells) doInsertAtPerf(pos term.Coordinates, r rune, width, byteCount uint8) {
	// make sure we have enough capacity
	c.cells[pos.Y] = append(c.cells[pos.Y], term.Cell{})
	copy(c.cells[pos.Y][pos.X+1:], c.cells[pos.Y][pos.X:])
	cell := term.Cell{Ch: r, Width: width, Bytes: byteCount}
	c.cells[pos.Y][pos.X] = cell
}

// specialized, most common case for perf improvements
func (c *rawCells) insertAtPerf(pos term.Coordinates, r rune, width uint8, byteCount uint8) (
	next term.Coordinates,
) {
	switch r {
	case '\u200d':
		next = term.Coordinates{X: pos.X + 1, Y: pos.Y}
		c.zwj = true
		c.doInsertAtPerf(pos, r, width, byteCount)
		c.zwjPos = next
		return
	case '\n':
		c.insertNewRow(pos)
		next = term.Coordinates{X: 0, Y: pos.Y + 1}
	default:
		at := pos
		c.doInsertAtPerf(at, r, width, byteCount)
		next = term.Coordinates{X: at.X + 1, Y: at.Y}
	}

	if c.zwj {
		if c.zwjPos == pos {
			str := c.String()
			c.reset()
			_, _ = c.readFromWithView(strings.NewReader(str), c)
			next = term.Coordinates{X: pos.X, Y: pos.Y}
		}
		c.zwj = false
		c.zwjPos = term.Coordinates{}
	}

	return
}

func (c *rawCells) fillInRows(y int) (n int) {
	for y >= len(c.cells) {
		n++
		row := makeNewRow(0, c.columnCap)
		c.cells = append(c.cells, row)
	}
	return
}

func (c *rawCells) fillInColumns(pos term.Coordinates) (n int) {
	for pos.X > len(c.cells[pos.Y]) {
		c.cells[pos.Y] = append(c.cells[pos.Y], term.Cell{Ch: c.fillInChar})
		n++
	}
	return
}

// extendRowToWidth pads row y with term.Cell{Ch: c.fillInChar} until its
// length reaches width. It performs a single slice grow when capacity is
// insufficient. If the row is already at least width columns wide, it is
// left unchanged.
func (c *rawCells) extendRowToWidth(y, width int) (added int) {
	if y < 0 || y >= len(c.cells) {
		return 0
	}
	row := c.cells[y]
	cur := len(row)
	if cur >= width {
		return 0
	}
	need := width - cur
	if cap(row) < width {
		grown := make([]term.Cell, width)
		copy(grown, row)
		row = grown
	} else {
		row = row[:width]
	}
	fill := term.Cell{Ch: c.fillInChar}
	for i := cur; i < width; i++ {
		row[i] = fill
	}
	c.cells[y] = row
	return need
}

// trimRowsFromEnd drops up to count rows from the end of c.cells, keeping
// at least one row (matches the invariant maintained elsewhere). It is
// allocation-free.
func (c *rawCells) trimRowsFromEnd(count int) (removed int) {
	if count <= 0 {
		return 0
	}
	n := len(c.cells)
	if n <= 1 {
		return 0
	}
	removed = count
	removed = min(removed, n-1)
	c.cells = c.cells[:n-removed]
	return removed
}

func (c *rawCells) fillInCoords(pos term.Coordinates) (
	from, to term.Coordinates, rowsFilled int,
) {
	assertValidCoords(pos)
	from = pos
	if rowsFilled = c.fillInRows(pos.Y); rowsFilled != 0 {
		from.Y -= rowsFilled
		// if rawCells was un-initialized or for some
		// reason base row was removed i.e. a truncate op
		if from.Y < 0 {
			from.Y = 0
		}
		from.X = c.Columns(from.Y)
		c.fillInColumns(pos)
	} else {
		from.X -= c.fillInColumns(pos)
	}
	to = pos

	return
}

func (c *rawCells) insert(at term.Coordinates, str string) (
	from, to term.Coordinates,
) {
	var rowsFilled int
	from, to, rowsFilled = c.fillInCoords(at)
	// fillInCoords fills in with newlines up to Y
	if rowsFilled != 0 {
		var i int
		for ; i < len(str) && str[i] == '\n'; i++ {
		}
		str = str[i:]
	}
	next := to
	state := -1
	var cluster string
	var width uint8
	for len(str) > 0 {
		cluster, str, width, state = graphemecluster.StepString(str, state)
		bytecount := uint8(len([]byte(cluster)))
		if bytecount == 1 { // specialized perf case for ASCII, avoids allocs
			for _, r := range cluster { // extract rune with no allocs
				next = c.insertAtPerf(next, r, width, bytecount)
			}
		} else {
			next = c.insertAt(next, []rune(cluster), width, bytecount)
		}
	}
	to = next
	return
}

func copyToBuilder(builder *strings.Builder, cells [][]term.Cell) {
	for i, r := range cells {
		if i != 0 {
			builder.WriteByte('\n')
		}
		copyRowToBuilder(builder, r)
	}
}

func copyRowToBuilder(builder *strings.Builder, cells []term.Cell) {
	builder.Grow(len(cells)) // almost every time this is exact
	for _, c := range cells {
		builder.WriteRune(c.Ch)
		for _, comb := range c.CombiningRunes() {
			builder.WriteRune(comb)
		}
	}
}

func copyToBuffer(builder *bytes.Buffer, cells [][]term.Cell) {
	for i, r := range cells {
		if i != 0 {
			builder.WriteByte('\n')
		}
		copyRowToBuffer(builder, r)
	}
}

func copyRowToBuffer(builder *bytes.Buffer, cells []term.Cell) {
	builder.Grow(len(cells)) // almost every time this is exact
	for _, c := range cells {
		builder.WriteRune(c.Ch)
		for _, comb := range c.CombiningRunes() {
			builder.WriteRune(comb)
		}
	}
}

func (c *rawCells) conflate(row int) {
	// copy cells from next row into current row
	rlen := len(c.cells[row+1])
	if rlen != 0 {
		c.cells[row] = append(c.cells[row], c.cells[row+1]...)
	}

	// copy all rows into row we just moved up and trim last row
	copy(c.cells[row+1:], c.cells[row+2:])
	c.cells = c.cells[:len(c.cells)-1]
}

// mergeMarkedRows walks rows [0..end] and, whenever a row's last cell
// satisfies isMark, merges it (and as many consecutive subsequent rows whose
// previous row's last cell satisfies isMark) into the group head. The merge
// is performed in a single pass across the backing slice.
//
// The mark cell in merged positions is cleared via clearMark before merging
// so the caller can use a sentinel byte (e.g. vte wrapMarker) to detect group
// continuation. end is exclusive; only rows at indexes < end are eligible to
// start a group (groups may still extend past end into later rows if marked).
//
// Returns the total number of rows consumed by merges (i.e. how much the
// row count shrank).
func (c *rawCells) mergeMarkedRows(
	end int,
	isMark func(term.Cell) bool,
	clearMark func(*term.Cell),
) (merged int) {
	n := len(c.cells)
	if end > n {
		end = n
	}
	if end <= 0 || n <= 1 {
		return 0
	}

	write := 0
	for read := 0; read < n; {
		row := c.cells[read]
		// Start of a potential group: check if this row is eligible (i.e.
		// head row must be within [0, end)) and its last cell is marked.
		lastCol := len(row) - 1
		if read >= end || lastCol < 0 || !isMark(row[lastCol]) {
			// No group starts here; just copy if write != read.
			if write != read {
				c.cells[write] = row
			}
			write++
			read++
			continue
		}

		// Collect consecutive rows that participate in this group.
		groupEnd := read
		for groupEnd+1 < n {
			curRow := c.cells[groupEnd]
			curLast := len(curRow) - 1
			if curLast < 0 || !isMark(curRow[curLast]) {
				break
			}
			groupEnd++
		}
		// groupEnd is inclusive last row of the merged line.

		// Compute total length and clear marks on intermediate rows.
		total := len(row)
		clearMark(&row[lastCol])
		for y := read + 1; y <= groupEnd; y++ {
			nextRow := c.cells[y]
			if last := len(nextRow) - 1; last >= 0 && y < groupEnd && isMark(nextRow[last]) {
				clearMark(&nextRow[last])
			}
			total += len(nextRow)
		}

		// Merge.
		merge := groupEnd - read
		if merge == 0 {
			if write != read {
				c.cells[write] = row
			}
			write++
			read++
			continue
		}

		if cap(row) < total {
			grown := make([]term.Cell, len(row), total)
			copy(grown, row)
			row = grown
		}
		for y := read + 1; y <= groupEnd; y++ {
			row = append(row, c.cells[y]...)
		}
		c.cells[write] = row
		write++
		read = groupEnd + 1
		merged += merge
	}
	c.cells = c.cells[:write]
	return merged
}

func (c *rawCells) deleteRowRange(
	builder *strings.Builder, row, fromX, toX int,
) {
	copyRowToBuilder(builder, c.cells[row][fromX:toX])
	diff := toX - fromX
	copy(c.cells[row][fromX:], c.cells[row][toX:])
	c.cells[row] = c.cells[row][:len(c.cells[row])-diff]
}

func (c *rawCells) delete(from, to term.Coordinates) (
	start, end term.Coordinates, str string,
) {
	assertValidCoords(from)
	assertValidCoords(to)
	start, end = term.CoordinatesSort(from, to)

	builder := strings.Builder{}

	if start.Y == end.Y {
		c.deleteRowRange(&builder, start.Y, start.X, end.X)
		str = builder.String()
		return
	}

	// trim til end of first row
	if start.X < len(c.cells[start.Y]) {
		c.deleteRowRange(&builder, start.Y, start.X, len(c.cells[start.Y]))
	}
	if start.Y+1 < len(c.cells) {
		builder.WriteByte('\n')
	}

	// copy rows in between and move last row to second row, if applicable
	lastRow := end.Y
	if diff := end.Y - start.Y; diff > 1 {
		copyToBuilder(&builder, c.cells[start.Y+1:end.Y])
		copy(c.cells[start.Y+1:], c.cells[end.Y:])
		c.cells = c.cells[:len(c.cells)-diff+1]

		lastRow = start.Y + 1
		if lastRow < len(c.cells) {
			builder.WriteByte('\n')
		}
	}

	// then remove cells from last row; start.Y is now last row to delete
	if end.X > 0 {
		c.deleteRowRange(&builder, lastRow, 0, end.X)
	}

	// conflate last row in range
	if lastRow < len(c.cells) {
		c.conflate(start.Y)
	}

	str = builder.String()

	return
}

func (c *rawCells) Edit(_ context.Context, start, end term.Coordinates, str string) (
	from, to term.Coordinates, old string,
) {
	from = start
	to = start
	if start != end {
		from, _, old = c.delete(start, end)
		to = from
	}

	if str != "" {
		from, to = c.insert(from, str)
	}

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
	return term.CellsToString(c.cells)
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
	return c.readFromWithView(r, c)
}

func (c *rawCells) readFromWithView(r io.Reader, view View) (int64, error) {
	rowY := nextWrite(view).Y
	reader := bufio.NewReader(r)
	n := int64(0)
	for {
		str, err := reader.ReadString('\n')
		state := -1
		var cluster string
		var width, byteCount uint8
		n += int64(len([]byte(str)))
		for len(str) > 0 {
			// NOTE: this is significantly slower than, just ignoring grapheme clusters
			// but it should be ok as it's done once per file, and because calculating the width
			// is front loaded, it should amortize over long interactions on a particular file.
			cluster, str, width, state = graphemecluster.StepString(str, state)
			// most of the type we'll hit this branch, which performs no extra allocations
			// in particular, the conversio from cluster string to []rune causes 1 slice
			// per cell, in each file, which is quite bit of of overhead.
			if len(cluster) == 1 && width <= 1 {
				byteCount = 1
				switch cluster[0] {
				case '\n':
					c.cells = append(c.cells, makeNewRow(0, c.columnCap))
					rowY++
				default:
					cell := term.Cell{
						Ch:    rune(cluster[0]),
						Width: width,
						Bytes: byteCount,
					}
					c.cells[rowY] = append(c.cells[rowY], cell)
				}
			} else {
				byteCount = uint8(len([]byte(cluster)))
				r := []rune(cluster)
				switch r[0] {
				case '\n':
					c.cells = append(c.cells, makeNewRow(0, c.columnCap))
					rowY++
				default:
					cell := term.Cell{
						Ch:    r[0],
						Width: width,
						Bytes: byteCount,
					}
					if len(r) > 1 {
						cell.SetCombining(r[1:])
					}
					c.cells[rowY] = append(c.cells[rowY], cell)
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return n, err
		}
	}
}
