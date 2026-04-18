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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// buildPerfBuffer returns a Buffer initialised in performance mode whose rows
// match the given text (one row per string). Each row is given capacity equal
// to length so rowsContiguous-style tests see deterministic slices.
func buildPerfBuffer(rows ...string) *Buffer {
	b := new(Buffer)
	b.InitPerformance(0, 0, ' ')
	b.cells.cells = make([][]term.Cell, len(rows))
	for i, r := range rows {
		b.cells.cells[i] = stringToCells(r)
	}
	if len(rows) == 0 {
		b.cells.cells = [][]term.Cell{{}}
	}
	return b
}

func stringToCells(s string) []term.Cell {
	rs := []rune(s)
	out := make([]term.Cell, len(rs))
	for i, r := range rs {
		out[i] = term.Cell{Ch: r, Width: 1, Bytes: uint8(len(string(r)))}
	}
	return out
}

func cellsRowsAsStrings(b *Buffer) []string {
	out := make([]string, b.Rows())
	for y := 0; y < b.Rows(); y++ {
		row := b.RawCells()[y]
		runes := make([]rune, len(row))
		for x, c := range row {
			runes[x] = c.Ch
		}
		out[y] = string(runes)
	}
	return out
}

func TestExtendRowToWidth(t *testing.T) {
	t.Run("extends short row using fillInChar", func(t *testing.T) {
		b := buildPerfBuffer("ab")
		added, ok := b.ExtendRowToWidth(0, 5)
		require.True(t, ok)
		assert.Equal(t, 3, added)
		assert.Equal(t, []string{"ab   "}, cellsRowsAsStrings(b))
	})
	t.Run("no-op when row already wide enough", func(t *testing.T) {
		b := buildPerfBuffer("abcde")
		added, ok := b.ExtendRowToWidth(0, 3)
		require.True(t, ok)
		assert.Equal(t, 0, added)
		assert.Equal(t, []string{"abcde"}, cellsRowsAsStrings(b))
	})
	t.Run("out of bounds row", func(t *testing.T) {
		b := buildPerfBuffer("ab")
		added, ok := b.ExtendRowToWidth(5, 8)
		require.True(t, ok)
		assert.Equal(t, 0, added)
	})
	t.Run("rejects non-performance mode", func(t *testing.T) {
		b := NewBuffer()
		assert.Panics(t, func() {
			_, _ = b.ExtendRowToWidth(0, 8)
		})
	})
	t.Run("uses custom fill char", func(t *testing.T) {
		b := new(Buffer)
		b.InitPerformance(0, 0, '.')
		b.cells.cells = [][]term.Cell{stringToCells("ab")}
		added, ok := b.ExtendRowToWidth(0, 4)
		require.True(t, ok)
		assert.Equal(t, 2, added)
		assert.Equal(t, []string{"ab.."}, cellsRowsAsStrings(b))
	})
}

func TestTrimRowsFromEnd(t *testing.T) {
	t.Run("trims requested rows", func(t *testing.T) {
		b := buildPerfBuffer("a", "b", "c", "d")
		removed, ok := b.TrimRowsFromEnd(2)
		require.True(t, ok)
		assert.Equal(t, 2, removed)
		assert.Equal(t, []string{"a", "b"}, cellsRowsAsStrings(b))
	})
	t.Run("keeps at least one row", func(t *testing.T) {
		b := buildPerfBuffer("a", "b")
		removed, ok := b.TrimRowsFromEnd(10)
		require.True(t, ok)
		assert.Equal(t, 1, removed)
		assert.Equal(t, []string{"a"}, cellsRowsAsStrings(b))
	})
	t.Run("no-op on single row buffer", func(t *testing.T) {
		b := buildPerfBuffer("a")
		removed, ok := b.TrimRowsFromEnd(1)
		require.True(t, ok)
		assert.Equal(t, 0, removed)
	})
	t.Run("rejects non-positive count", func(t *testing.T) {
		b := buildPerfBuffer("a", "b")
		removed, ok := b.TrimRowsFromEnd(0)
		require.True(t, ok)
		assert.Equal(t, 0, removed)
	})
	t.Run("rejects non-performance mode", func(t *testing.T) {
		b := NewBuffer()
		assert.Panics(t, func() {
			_, _ = b.TrimRowsFromEnd(1)
		})
	})
}

const testMark uint8 = 0x80

func markLastCell(cells []term.Cell) {
	if len(cells) == 0 {
		return
	}
	cells[len(cells)-1].Bytes = testMark
}

func TestMergeMarkedRows(t *testing.T) {
	isMark := func(c term.Cell) bool { return c.Bytes == testMark }
	clearMark := func(c *term.Cell) { c.Bytes = 0 }
	t.Run("merges a single two-row group", func(t *testing.T) {
		b := buildPerfBuffer("abc", "def", "gh")
		markLastCell(b.cells.cells[0]) // "abc" continues to "def"
		merged, ok := b.MergeMarkedRows(3, isMark, clearMark)
		require.True(t, ok)
		assert.Equal(t, 1, merged)
		assert.Equal(t, []string{"abcdef", "gh"}, cellsRowsAsStrings(b))
		assert.Equal(t, uint8(0), b.cells.cells[0][2].Bytes,
			"mark should be cleared on merged boundary")
	})
	t.Run("merges a chain of marked rows", func(t *testing.T) {
		b := buildPerfBuffer("ab", "cd", "ef", "gh")
		markLastCell(b.cells.cells[0])
		markLastCell(b.cells.cells[1])
		markLastCell(b.cells.cells[2])
		merged, ok := b.MergeMarkedRows(4, isMark, clearMark)
		require.True(t, ok)
		assert.Equal(t, 3, merged)
		assert.Equal(t, []string{"abcdefgh"}, cellsRowsAsStrings(b))
	})
	t.Run("does not start a group past end", func(t *testing.T) {
		b := buildPerfBuffer("ab", "cd", "ef")
		markLastCell(b.cells.cells[1]) // "cd" marks, but head is at y=1 >= end=1
		merged, ok := b.MergeMarkedRows(1, isMark, clearMark)
		require.True(t, ok)
		assert.Equal(t, 0, merged)
		assert.Equal(t, []string{"ab", "cd", "ef"}, cellsRowsAsStrings(b))
	})
	t.Run("ignores rows with no cells", func(t *testing.T) {
		b := buildPerfBuffer("ab", "", "cd")
		merged, ok := b.MergeMarkedRows(3, isMark, clearMark)
		require.True(t, ok)
		assert.Equal(t, 0, merged)
	})
	t.Run("merges through a contiguous slab", func(t *testing.T) {
		// Simulate what copyCellsContiguous produces: all rows share a
		// single backing slab with cap==len. This should trigger the
		// unsafe fast path in mergeMarkedRows.
		src := [][]term.Cell{
			stringToCells("ab"),
			stringToCells("cd"),
			stringToCells("ef"),
		}
		b := new(Buffer)
		b.InitPerformance(0, 0, ' ')
		b.cells.cells = copyCellsContiguous(b.cells.cells, src)
		markLastCell(b.cells.cells[0])
		markLastCell(b.cells.cells[1])
		merged, ok := b.MergeMarkedRows(3, isMark, clearMark)
		require.True(t, ok)
		assert.Equal(t, 2, merged)
		assert.Equal(t, []string{"abcdef"}, cellsRowsAsStrings(b))
	})
	t.Run("rejects non-performance mode", func(t *testing.T) {
		b := NewBuffer()
		assert.Panics(t, func() {
			_, _ = b.MergeMarkedRows(1, isMark, clearMark)
		})
	})
}

func TestSplitRowsBatch(t *testing.T) {
	t.Run("splits single row into two pieces", func(t *testing.T) {
		b := buildPerfBuffer("abcdefgh")
		added, ok := b.SplitRowsBatch([]RowSplit{{Y: 0, Width: 4, Times: 1}})
		require.True(t, ok)
		assert.Equal(t, 1, added)
		assert.Equal(t, []string{"abcd", "efgh"}, cellsRowsAsStrings(b))
	})
	t.Run("splits single row into three pieces", func(t *testing.T) {
		b := buildPerfBuffer("abcdefghij")
		added, ok := b.SplitRowsBatch([]RowSplit{{Y: 0, Width: 3, Times: 2}})
		require.True(t, ok)
		assert.Equal(t, 2, added)
		// expected: "abc", "def", "ghij"
		assert.Equal(t, []string{"abc", "def", "ghij"}, cellsRowsAsStrings(b))
	})
	t.Run("multiple splits in one batch preserve other rows", func(t *testing.T) {
		b := buildPerfBuffer("AAAA", "keep1", "BBBB", "keep2")
		added, ok := b.SplitRowsBatch([]RowSplit{
			{Y: 0, Width: 2, Times: 1},
			{Y: 2, Width: 2, Times: 1},
		})
		require.True(t, ok)
		assert.Equal(t, 2, added)
		assert.Equal(t, []string{
			"AA", "AA", "keep1", "BB", "BB", "keep2",
		}, cellsRowsAsStrings(b))
	})
	t.Run("empty splits is a no-op", func(t *testing.T) {
		b := buildPerfBuffer("abc", "def")
		added, ok := b.SplitRowsBatch(nil)
		require.True(t, ok)
		assert.Equal(t, 0, added)
		assert.Equal(t, []string{"abc", "def"}, cellsRowsAsStrings(b))
	})
	t.Run("pieces do not share writable capacity", func(t *testing.T) {
		// Appending to the head after a split must not clobber the tail
		// piece's backing memory.
		b := buildPerfBuffer("abcdefgh")
		_, _ = b.SplitRowsBatch([]RowSplit{{Y: 0, Width: 4, Times: 1}})
		b.cells.cells[0] = append(b.cells.cells[0], term.Cell{Ch: 'Z'})
		assert.Equal(t, []string{"abcdZ", "efgh"}, cellsRowsAsStrings(b))
	})
	t.Run("rejects non-performance mode", func(t *testing.T) {
		b := NewBuffer()
		assert.Panics(t, func() {
			_, _ = b.SplitRowsBatch([]RowSplit{{Y: 0, Width: 1, Times: 1}})
		})
	})
}

func TestSplitRowsBatchPadded(t *testing.T) {
	t.Run("pads short tail with fill char", func(t *testing.T) {
		b := buildPerfBuffer("abcde")
		added, ok := b.SplitRowsBatchPadded(
			[]RowSplit{{Y: 0, Width: 3, Times: 1}}, 3, '.')
		require.True(t, ok)
		assert.Equal(t, 1, added)
		assert.Equal(t, []string{"abc", "de."}, cellsRowsAsStrings(b))
	})
	t.Run("leaves exact-width tail untouched", func(t *testing.T) {
		b := buildPerfBuffer("abcdef")
		_, ok := b.SplitRowsBatchPadded(
			[]RowSplit{{Y: 0, Width: 3, Times: 1}}, 3, '.')
		require.True(t, ok)
		assert.Equal(t, []string{"abc", "def"}, cellsRowsAsStrings(b))
	})
	t.Run("works with padToWidth=0 (same as unpadded)", func(t *testing.T) {
		b := buildPerfBuffer("abcde")
		_, ok := b.SplitRowsBatchPadded(
			[]RowSplit{{Y: 0, Width: 3, Times: 1}}, 0, '.')
		require.True(t, ok)
		assert.Equal(t, []string{"abc", "de"}, cellsRowsAsStrings(b))
	})
	t.Run("pads multiple short tails from single slab", func(t *testing.T) {
		b := buildPerfBuffer("abcXY", "def123")
		_, ok := b.SplitRowsBatchPadded([]RowSplit{
			{Y: 0, Width: 3, Times: 1},
			{Y: 1, Width: 3, Times: 1},
		}, 3, '.')
		require.True(t, ok)
		assert.Equal(t, []string{"abc", "XY.", "def", "123"}, cellsRowsAsStrings(b))
	})
	t.Run("rejects non-performance mode", func(t *testing.T) {
		b := NewBuffer()
		assert.Panics(t, func() {
			_, _ = b.SplitRowsBatchPadded(
				[]RowSplit{{Y: 0, Width: 1, Times: 1}}, 2, ' ')
		})
	})
}

func TestCopyCellsContiguous(t *testing.T) {
	t.Run("produces rows with cap==len, contiguous in memory", func(t *testing.T) {
		src := [][]term.Cell{
			stringToCells("abc"),
			stringToCells("de"),
			stringToCells("fghi"),
		}
		dst := copyCellsContiguous(nil, src)
		require.Len(t, dst, 3)
		for i, want := range []string{"abc", "de", "fghi"} {
			assert.Equal(t, len(want), len(dst[i]))
			assert.Equal(t, len(want), cap(dst[i]))
		}
	})
	t.Run("empty rows become nil", func(t *testing.T) {
		src := [][]term.Cell{stringToCells("ab"), {}, stringToCells("cd")}
		dst := copyCellsContiguous(nil, src)
		require.Len(t, dst, 3)
		assert.Nil(t, dst[1])
	})
	t.Run("empty src yields empty dst rows", func(t *testing.T) {
		src := [][]term.Cell{{}, {}}
		dst := copyCellsContiguous(nil, src)
		require.Len(t, dst, 2)
		assert.Nil(t, dst[0])
		assert.Nil(t, dst[1])
	})
}

func TestConflatePerfModeFastPath(t *testing.T) {
	t.Run("conflates two rows without allocating", func(t *testing.T) {
		b := buildPerfBuffer("abc", "def")
		x, ok := b.ConflateRow(0)
		require.True(t, ok)
		assert.Equal(t, 3, x)
		assert.Equal(t, []string{"abcdef"}, cellsRowsAsStrings(b))
	})
	t.Run("rejects conflate on last row", func(t *testing.T) {
		b := buildPerfBuffer("abc", "def")
		_, ok := b.ConflateRow(1)
		assert.False(t, ok)
	})
	t.Run("rejects conflate on single-row buffer", func(t *testing.T) {
		b := buildPerfBuffer("abc")
		_, ok := b.ConflateRow(0)
		assert.False(t, ok)
	})
}

func TestWrapRowPerfModeFastPath(t *testing.T) {
	t.Run("wraps a row at a column, keeping cells on both sides", func(t *testing.T) {
		b := buildPerfBuffer("abcdefgh")
		ok := b.WrapRow(0, 4)
		require.True(t, ok)
		assert.Equal(t, []string{"abcd", "efgh"}, cellsRowsAsStrings(b))
	})
	t.Run("wraps at end of row creates trailing empty row", func(t *testing.T) {
		b := buildPerfBuffer("abcd")
		ok := b.WrapRow(0, 4)
		require.True(t, ok)
		assert.Equal(t, 2, b.Rows())
		assert.Equal(t, 4, b.Columns(0))
		assert.Equal(t, 0, b.Columns(1))
	})
	t.Run("rejects out of bounds at", func(t *testing.T) {
		b := buildPerfBuffer("abcd")
		ok := b.WrapRow(0, 5)
		assert.False(t, ok)
	})
	t.Run("rejects out of bounds row", func(t *testing.T) {
		b := buildPerfBuffer("abcd")
		ok := b.WrapRow(1, 0)
		assert.False(t, ok)
	})
}

func TestWrapConflateRoundTrip(t *testing.T) {
	b := buildPerfBuffer("abcdefgh")
	require.True(t, b.WrapRow(0, 4))
	x, ok := b.ConflateRow(0)
	require.True(t, ok)
	assert.Equal(t, 4, x)
	assert.Equal(t, []string{"abcdefgh"}, cellsRowsAsStrings(b))
}

func TestCellsToBufferPerformance(t *testing.T) {
	t.Run("preserves row contents", func(t *testing.T) {
		src := [][]term.Cell{
			stringToCells("abc"),
			stringToCells("de"),
			stringToCells("fghi"),
		}
		b := CellsToBufferPerformance(src, ' ')
		assert.Equal(t, []string{"abc", "de", "fghi"}, cellsRowsAsStrings(b))
	})
	t.Run("initializes with at least one row", func(t *testing.T) {
		b := CellsToBufferPerformance(nil, ' ')
		assert.Equal(t, 1, b.Rows())
	})
	t.Run("uses fillInChar for later extend", func(t *testing.T) {
		b := CellsToBufferPerformance([][]term.Cell{stringToCells("ab")}, '.')
		added, ok := b.ExtendRowToWidth(0, 5)
		require.True(t, ok)
		assert.Equal(t, 3, added)
		assert.Equal(t, []string{"ab..."}, cellsRowsAsStrings(b))
	})
	t.Run("is in performance mode (no undo)", func(t *testing.T) {
		b := CellsToBufferPerformance([][]term.Cell{stringToCells("a")}, ' ')
		assert.Nil(t, b.undoer,
			"CellsToBufferPerformance should not allocate an undoer")
	})
	t.Run("wrap then conflate round trips correctly", func(t *testing.T) {
		b := CellsToBufferPerformance([][]term.Cell{stringToCells("abcdefgh")}, ' ')
		require.True(t, b.WrapRow(0, 4))
		assert.Equal(t, []string{"abcd", "efgh"}, cellsRowsAsStrings(b))
		_, ok := b.ConflateRow(0)
		require.True(t, ok)
		assert.Equal(t, []string{"abcdefgh"}, cellsRowsAsStrings(b))
	})
	t.Run("supports MergeMarkedRows across copied slab", func(t *testing.T) {
		src := [][]term.Cell{
			stringToCells("ab"),
			stringToCells("cd"),
			stringToCells("ef"),
		}
		b := CellsToBufferPerformance(src, ' ')
		markLastCell(b.cells.cells[0])
		markLastCell(b.cells.cells[1])
		merged, ok := b.MergeMarkedRows(3,
			func(c term.Cell) bool { return c.Bytes == testMark },
			func(c *term.Cell) { c.Bytes = 0 })
		require.True(t, ok)
		assert.Equal(t, 2, merged)
		assert.Equal(t, []string{"abcdef"}, cellsRowsAsStrings(b))
	})
}
