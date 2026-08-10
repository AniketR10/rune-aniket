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
	"bytes"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// CellsToBytesBuffer copies the bytes representation of the given cell matrix
// to the supplied buffer.
//
// Caller is responsible for resetting buffer prior to this call if necessary.
func CellsToBytesBuffer(buffer *bytes.Buffer, cells [][]term.Cell) {
	copyToBuffer(buffer, cells)
}

// CellsToBuffer efficienty returns a Buffer that uses c as the
// underlying matrix of cells.
//
// Note that this buffer will honor the tabspaces observed in c.
// If c does not have any tabspaces, then 1 tabspace is assumed.
//
// Furthermore, it won't treat the last EOL as mandatory so it can be used
// as an in-memory buffer.
func CellsToBuffer(c [][]term.Cell) *Buffer {
	return cellsToBuffer(c, ' ', false)
}

// CellsToBufferPerformance is like CellsToBuffer, but initializes the returned
// buffer in performance mode and uses fillInChar when future edits need to fill
// rows or columns beyond the current content.
func CellsToBufferPerformance(c [][]term.Cell, fillInChar rune) *Buffer {
	return cellsToBuffer(c, fillInChar, true)
}

// AdoptCellsToBuffer returns a Buffer that takes ownership of c as its
// underlying matrix without copying. The caller must not retain, alias
// or mutate c or its rows after the call. Use when the rows were built
// specifically for the Buffer (e.g. decoded from an RPC payload) and a
// defensive copy would only add allocation churn.
func AdoptCellsToBuffer(c [][]term.Cell) *Buffer {
	cells := new(rawCells)
	cells.setFillInChar(' ')
	cells.columnCap = defColumnCap
	cells.rowCap = defRowCap
	cells.cells = c
	// rawCells has the property that there's always at least one row.
	if cells.Rows() == 0 {
		cells.fillInRows(0)
	}
	ret := new(Buffer)
	ret.initWithCells(cells)
	return ret
}

func cellsToBuffer(c [][]term.Cell, fillInChar rune, performance bool) *Buffer {
	cells := new(rawCells)
	cells.setFillInChar(fillInChar)
	cells.resetWithCap(defRowCap, defColumnCap)
	if performance {
		cells.cells = copyCellsContiguous(cells.cells, c)
	} else {
		cells.cells = term.CopyCells(cells.cells, c)
	}

	// rawCells has the property that there's always at least one row.
	if cells.Rows() == 0 {
		cells.fillInRows(0)
	}

	ret := new(Buffer)
	if performance {
		ret.initPerformanceWithCells(cells)
	} else {
		ret.initWithCells(cells)
	}
	return ret
}

// copyCellsContiguous copies src into a destination row slice where every
// copied row is sliced from a single contiguous backing array with
// cap == len. This enables copy-free merges (rowsContiguous fast path) when
// later editing coalesces adjacent rows.
//
// Individual row appends that exceed a row's cap will still reallocate that
// row in Go's normal way, at which point the contiguity invariant for that
// row is broken — but neighbouring untouched rows remain contiguous with
// each other, so the fast path still fires for them.
func copyCellsContiguous(dst [][]term.Cell, src [][]term.Cell) [][]term.Cell {
	if len(dst) > len(src) {
		dst = dst[:len(src)]
	} else if len(dst) < len(src) {
		for i := len(dst); i < len(src); i++ {
			dst = append(dst, nil)
		}
	}
	var total int
	for _, r := range src {
		total += len(r)
	}
	if total == 0 {
		for i := range src {
			dst[i] = nil
		}
		return dst
	}
	slab := make([]term.Cell, total)
	var off int
	for i, r := range src {
		n := len(r)
		if n == 0 {
			dst[i] = nil
			continue
		}
		copy(slab[off:off+n], r)
		// Use 3-index slicing so cap == len and subsequent rows start right
		// after the previous one's end.
		dst[i] = slab[off : off+n : off+n]
		off += n
	}
	return dst
}

// ConvertRunePosToCoordinates converts the given column and row position in number of
// bytes, into code-point coordinates.
func ConvertRunePosToCoordinates(cells [][]term.Cell, y, x int) (
	ret term.Coordinates, ok bool,
) {
	return runePosToCoordinates(cells, cellByteCount, y, x)
}

// ConvertCoordinatesToRunePos converts the given coordinates, which
// represent code point coordinates into row, column position in numbers of bytes.
func ConvertCoordinatesToRunePos(cells [][]term.Cell, c term.Coordinates) (
	y, x int, ok bool,
) {
	return coordinatesToRunePos(cells, cellByteCount, c)
}

// ConvertByteOffsetToCoordinates converts the given byte offsets to term.Coordinates.
// It returns false if it's out of bounds.
func ConvertByteOffsetToCoordinates(cells [][]term.Cell, offset int) (
	ret term.Coordinates, ok bool,
) {
	return byteOffsetToCoordinates(cells, cellByteCount, offset)
}

// ConvertCoordinatesToByteOffset converts the given coordinates to a byte offset.
// It returns false if it's out of bounds.
func ConvertCoordinatesToByteOffset(cells [][]term.Cell, c term.Coordinates) (
	offset int, ok bool,
) {
	return coordinatesToByteOffset(cells, cellByteCount, c)
}

// ByteCounts is a compact snapshot of a cell matrix retaining only each
// cell's byte size (term.Cell.Bytes). It supports the same
// coordinate/byte-offset conversions as a full [][]term.Cell at 1 byte
// per cell instead of 24, for holders that must keep a point-in-time
// copy of the buffer's shape (e.g. syntax trees) without duplicating
// the whole cell matrix.
type ByteCounts [][]uint8

// NewByteCounts snapshots cells into dst, reusing dst's outer slice
// capacity. Rows are carved exact-size out of a single slab.
func NewByteCounts(dst ByteCounts, cells [][]term.Cell) ByteCounts {
	dst = dst[:0]
	var total int
	for _, r := range cells {
		total += len(r)
	}
	slab := make([]uint8, total)
	var off int
	for _, r := range cells {
		n := len(r)
		row := slab[off : off+n : off+n]
		off += n
		for x := range r {
			row[x] = r[x].Bytes
		}
		dst = append(dst, row)
	}
	return dst
}

// RunePosToCoordinates is the ByteCounts form of ConvertRunePosToCoordinates.
func (b ByteCounts) RunePosToCoordinates(y, x int) (term.Coordinates, bool) {
	return runePosToCoordinates(b, uint8ByteCount, y, x)
}

// CoordinatesToRunePos is the ByteCounts form of ConvertCoordinatesToRunePos.
func (b ByteCounts) CoordinatesToRunePos(c term.Coordinates) (y, x int, ok bool) {
	return coordinatesToRunePos(b, uint8ByteCount, c)
}

// ByteOffsetToCoordinates is the ByteCounts form of ConvertByteOffsetToCoordinates.
func (b ByteCounts) ByteOffsetToCoordinates(offset int) (term.Coordinates, bool) {
	return byteOffsetToCoordinates(b, uint8ByteCount, offset)
}

// CoordinatesToByteOffset is the ByteCounts form of ConvertCoordinatesToByteOffset.
func (b ByteCounts) CoordinatesToByteOffset(c term.Coordinates) (int, bool) {
	return coordinatesToByteOffset(b, uint8ByteCount, c)
}

func cellByteCount(c term.Cell) int { return int(c.Bytes) }

func uint8ByteCount(b uint8) int { return int(b) }

func runePosToCoordinates[T any](cells [][]T, bytes func(T) int, y, x int) (
	ret term.Coordinates, ok bool,
) {
	if len(cells) == 0 {
		return
	}

	if y > len(cells) {
		y = len(cells)
	}

	ret.Y = y
	if ret.Y == len(cells) {
		ok = true
		return
	}

	cellView := [1][]T{cells[ret.Y]}
	var bret term.Coordinates
	bret, ok = byteOffsetToCoordinates(cellView[:], bytes, x)
	if !ok {
		return
	}
	ret.X = bret.X
	return
}

func coordinatesToRunePos[T any](cells [][]T, bytes func(T) int, c term.Coordinates) (
	y, x int, ok bool,
) {
	if c.Y < 0 || c.X < 0 {
		panic("negative coordinates")
	}
	if len(cells) == 0 {
		return
	}

	if c.Y > len(cells) {
		c.Y = len(cells)
	}
	y = c.Y
	if c.Y == len(cells) {
		ok = true
		return
	}

	line := cells[c.Y]
	if c.X > len(line) {
		c.X = len(line)
	}
	cellView := [1][]T{line}
	var bretX int
	bretX, ok = coordinatesToByteOffset(cellView[:], bytes, term.Coordinates{X: c.X})
	if !ok {
		return
	}
	x = bretX
	return
}

func byteOffsetToCoordinates[T any](cells [][]T, bytes func(T) int, offset int) (
	ret term.Coordinates, ok bool,
) {
	if offset < 0 {
		panic("negative offset")
	}
	if len(cells) == 0 {
		return term.Coordinates{}, false
	}
	pos := 0
	for y, row := range cells {
		if pos >= offset {
			return term.Coordinates{X: 0, Y: y}, true
		}
		for x := range row {
			pos += bytes(row[x])
			if pos >= offset {
				return term.Coordinates{X: x + 1, Y: y}, true
			}
		}
		if pos == offset {
			return term.Coordinates{X: len(row), Y: y}, true
		}
		if y+1 < len(cells) {
			pos++
		}
	}
	return term.Coordinates{X: 0, Y: len(cells)}, true
}

func coordinatesToByteOffset[T any](cells [][]T, bytes func(T) int, c term.Coordinates) (
	offset int, ok bool,
) {
	if c.Y < 0 || c.X < 0 {
		panic("negative coordinates")
	}
	if c == (term.Coordinates{}) {
		return 0, len(cells) >= 1
	}
	var row []T
	if c.Y >= len(cells) {
		c.Y = len(cells)
	} else {
		row = cells[c.Y]
	}
	if c.X > len(row) {
		c.X = len(row)
	}
	for y := 0; y < c.Y; y++ {
		for _, cell := range cells[y] {
			offset += bytes(cell)
		}
		offset++ // \n
	}
	for x := 0; x < c.X; x++ {
		offset += bytes(row[x])
	}
	return offset, true
}

func nextWrite(c View) term.Coordinates {
	y := c.Rows() - 1
	x := len(c.RawCells()[y])
	return term.Coordinates{X: x, Y: y}
}
