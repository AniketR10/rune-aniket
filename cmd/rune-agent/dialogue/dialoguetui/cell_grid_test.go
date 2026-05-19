// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package dialoguetui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func setGridText(g *cellGrid, y int, text string) {
	for x, ch := range text {
		if x >= g.width {
			break
		}
		g.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{Ch: ch})
	}
}

func TestCellGridTextBetween(t *testing.T) {
	t.Run("single line", func(t *testing.T) {
		var g cellGrid
		g.Resize(20, 5)
		setGridText(&g, 0, "Hello World")

		text := g.TextBetween(
			term.Coordinates{X: 0, Y: 0},
			term.Coordinates{X: 4, Y: 0},
		)
		assert.Equal(t, "Hello", text)
	})

	t.Run("multi-line", func(t *testing.T) {
		var g cellGrid
		g.Resize(20, 5)
		setGridText(&g, 0, "line one")
		setGridText(&g, 1, "line two")

		text := g.TextBetween(
			term.Coordinates{X: 0, Y: 0},
			term.Coordinates{X: 7, Y: 1},
		)
		assert.Equal(t, "line one\nline two", text)
	})

	t.Run("trailing whitespace trimmed", func(t *testing.T) {
		var g cellGrid
		g.Resize(20, 5)
		setGridText(&g, 0, "hello   ")

		text := g.TextBetween(
			term.Coordinates{X: 0, Y: 0},
			term.Coordinates{X: 19, Y: 0},
		)
		assert.Equal(t, "hello", text)
	})

	t.Run("empty cells skipped", func(t *testing.T) {
		var g cellGrid
		g.Resize(20, 5)
		// Set cells with a gap (x=2 is empty)
		g.SetCell(term.Coordinates{X: 0, Y: 0}, term.Cell{Ch: 'A'})
		g.SetCell(term.Coordinates{X: 1, Y: 0}, term.Cell{Ch: 'B'})
		// x=2 is zero (empty)
		g.SetCell(term.Coordinates{X: 3, Y: 0}, term.Cell{Ch: 'C'})

		text := g.TextBetween(
			term.Coordinates{X: 0, Y: 0},
			term.Coordinates{X: 3, Y: 0},
		)
		assert.Equal(t, "ABC", text)
	})

	t.Run("trailing empty lines trimmed", func(t *testing.T) {
		var g cellGrid
		g.Resize(20, 5)
		setGridText(&g, 0, "hello")
		// rows 1-4 are empty

		text := g.TextBetween(
			term.Coordinates{X: 0, Y: 0},
			term.Coordinates{X: 19, Y: 2},
		)
		assert.Equal(t, "hello", text)
	})

	t.Run("reversed coordinates", func(t *testing.T) {
		var g cellGrid
		g.Resize(20, 5)
		setGridText(&g, 0, "Hello World")

		text := g.TextBetween(
			term.Coordinates{X: 4, Y: 0},
			term.Coordinates{X: 0, Y: 0},
		)
		assert.Equal(t, "Hello", text)
	})

	t.Run("combining runes", func(t *testing.T) {
		var g cellGrid
		g.Resize(20, 5)
		g.SetCell(term.Coordinates{X: 0, Y: 0}, term.Cell{
			Ch:        'e',
			Combining: &[]rune{0x0301}, // combining acute accent
		})
		g.SetCell(term.Coordinates{X: 1, Y: 0}, term.Cell{Ch: 'x'})

		text := g.TextBetween(
			term.Coordinates{X: 0, Y: 0},
			term.Coordinates{X: 1, Y: 0},
		)
		assert.Equal(t, "e\u0301x", text)
	})
}

func TestCellGridWordBoundsAt(t *testing.T) {
	t.Run("word chars", func(t *testing.T) {
		var g cellGrid
		g.Resize(20, 5)
		setGridText(&g, 0, "hello world")

		start, end, ok := g.WordBoundsAt(term.Coordinates{X: 2, Y: 0})
		assert.True(t, ok)
		assert.Equal(t, term.Coordinates{X: 0, Y: 0}, start)
		assert.Equal(t, term.Coordinates{X: 4, Y: 0}, end)
	})

	t.Run("second word", func(t *testing.T) {
		var g cellGrid
		g.Resize(20, 5)
		setGridText(&g, 0, "hello world")

		start, end, ok := g.WordBoundsAt(term.Coordinates{X: 8, Y: 0})
		assert.True(t, ok)
		assert.Equal(t, term.Coordinates{X: 6, Y: 0}, start)
		assert.Equal(t, term.Coordinates{X: 10, Y: 0}, end)
	})

	t.Run("punctuation", func(t *testing.T) {
		var g cellGrid
		g.Resize(20, 5)
		setGridText(&g, 0, "foo::bar")

		start, end, ok := g.WordBoundsAt(term.Coordinates{X: 3, Y: 0})
		assert.True(t, ok)
		assert.Equal(t, term.Coordinates{X: 3, Y: 0}, start)
		assert.Equal(t, term.Coordinates{X: 4, Y: 0}, end)
	})

	t.Run("whitespace returns false", func(t *testing.T) {
		var g cellGrid
		g.Resize(20, 5)
		setGridText(&g, 0, "hello world")

		_, _, ok := g.WordBoundsAt(term.Coordinates{X: 5, Y: 0})
		assert.False(t, ok)
	})

	t.Run("empty cell returns false", func(t *testing.T) {
		var g cellGrid
		g.Resize(20, 5)
		// All cells are zero

		_, _, ok := g.WordBoundsAt(term.Coordinates{X: 0, Y: 0})
		assert.False(t, ok)
	})

	t.Run("out of bounds returns false", func(t *testing.T) {
		var g cellGrid
		g.Resize(10, 5)

		_, _, ok := g.WordBoundsAt(term.Coordinates{X: 15, Y: 0})
		assert.False(t, ok)
	})

	t.Run("underscore is word char", func(t *testing.T) {
		var g cellGrid
		g.Resize(20, 5)
		setGridText(&g, 0, "foo_bar baz")

		start, end, ok := g.WordBoundsAt(term.Coordinates{X: 3, Y: 0})
		assert.True(t, ok)
		assert.Equal(t, term.Coordinates{X: 0, Y: 0}, start)
		assert.Equal(t, term.Coordinates{X: 6, Y: 0}, end)
	})
}

func TestCellGridLineBounds(t *testing.T) {
	t.Run("full row", func(t *testing.T) {
		var g cellGrid
		g.Resize(10, 5)
		setGridText(&g, 0, "0123456789")

		start, end, ok := g.LineBounds(0)
		assert.True(t, ok)
		assert.Equal(t, term.Coordinates{X: 0, Y: 0}, start)
		assert.Equal(t, term.Coordinates{X: 9, Y: 0}, end)
	})

	t.Run("partial row", func(t *testing.T) {
		var g cellGrid
		g.Resize(20, 5)
		// Place text starting at x=2 (leave x=0,1 as zero cells).
		for i, ch := range "hello" {
			g.SetCell(term.Coordinates{X: i + 2, Y: 1}, term.Cell{Ch: ch})
		}

		start, end, ok := g.LineBounds(1)
		assert.True(t, ok)
		assert.Equal(t, term.Coordinates{X: 2, Y: 1}, start)
		assert.Equal(t, term.Coordinates{X: 6, Y: 1}, end)
	})

	t.Run("empty row", func(t *testing.T) {
		var g cellGrid
		g.Resize(10, 5)

		_, _, ok := g.LineBounds(2)
		assert.False(t, ok)
	})

	t.Run("out of bounds row", func(t *testing.T) {
		var g cellGrid
		g.Resize(10, 5)

		_, _, ok := g.LineBounds(10)
		assert.False(t, ok)
	})
}

func TestCellGridDumpSelection(t *testing.T) {
	var g cellGrid
	g.Resize(10, 3)
	setGridText(&g, 0, "ABCDE")
	setGridText(&g, 1, "FGHIJ")

	sel := &selRange{
		start:  term.Coordinates{X: 1, Y: 0},
		end:    term.Coordinates{X: 3, Y: 0},
		active: true,
	}

	w := term.NewStringWriter(10, 3)
	g.Dump(w, sel)

	// Cells B, C, D (x=1,2,3 on y=0) should have AttrReverse.
	cells := w.Cells()
	assert.Equal(t, term.AttrReverse, cells[1].Attrs, "x=1 should have reverse")
	assert.Equal(t, term.AttrReverse, cells[2].Attrs, "x=2 should have reverse")
	assert.Equal(t, term.AttrReverse, cells[3].Attrs, "x=3 should have reverse")
	// x=0 should not have reverse
	assert.Equal(t, term.AttrMask(0), cells[0].Attrs, "x=0 should not have reverse")
	// x=4 should not have reverse
	assert.Equal(t, term.AttrMask(0), cells[4].Attrs, "x=4 should not have reverse")
}

func TestCellGridSelRangeContains(t *testing.T) {
	sel := selRange{
		start:  term.Coordinates{X: 2, Y: 1},
		end:    term.Coordinates{X: 5, Y: 3},
		active: true,
	}

	tests := []struct {
		pos  term.Coordinates
		want bool
	}{
		{term.Coordinates{X: 0, Y: 0}, false}, // above
		{term.Coordinates{X: 0, Y: 4}, false}, // below
		{term.Coordinates{X: 1, Y: 1}, false}, // first line, before start
		{term.Coordinates{X: 2, Y: 1}, true},  // first line, at start
		{term.Coordinates{X: 9, Y: 1}, true},  // first line, after start
		{term.Coordinates{X: 0, Y: 2}, true},  // middle line, any X
		{term.Coordinates{X: 0, Y: 3}, true},  // last line, before end
		{term.Coordinates{X: 5, Y: 3}, true},  // last line, at end
		{term.Coordinates{X: 6, Y: 3}, false}, // last line, after end
	}

	for _, tc := range tests {
		assert.Equal(t, tc.want, sel.Contains(tc.pos), "pos=%v", tc.pos)
	}

	t.Run("inactive range", func(t *testing.T) {
		inactive := selRange{
			start: term.Coordinates{X: 0, Y: 0},
			end:   term.Coordinates{X: 5, Y: 5},
		}
		assert.False(t, inactive.Contains(term.Coordinates{X: 2, Y: 2}))
	})

	t.Run("single line range", func(t *testing.T) {
		single := selRange{
			start:  term.Coordinates{X: 2, Y: 1},
			end:    term.Coordinates{X: 5, Y: 1},
			active: true,
		}
		assert.False(t, single.Contains(term.Coordinates{X: 1, Y: 1}))
		assert.True(t, single.Contains(term.Coordinates{X: 2, Y: 1}))
		assert.True(t, single.Contains(term.Coordinates{X: 5, Y: 1}))
		assert.False(t, single.Contains(term.Coordinates{X: 6, Y: 1}))
	})
}

func TestCellGridResize(t *testing.T) {
	var g cellGrid
	g.Resize(10, 5)
	setGridText(&g, 0, "hello")

	// Verify content is there.
	text := g.TextBetween(
		term.Coordinates{X: 0, Y: 0},
		term.Coordinates{X: 4, Y: 0},
	)
	assert.Equal(t, "hello", text)

	// Resize to same or smaller — should reuse allocation and clear.
	oldCap := cap(g.cells)
	g.Resize(10, 5)
	assert.Equal(t, oldCap, cap(g.cells), "should reuse allocation")

	text = g.TextBetween(
		term.Coordinates{X: 0, Y: 0},
		term.Coordinates{X: 4, Y: 0},
	)
	assert.Equal(t, "", text, "should be cleared after resize")
}

func TestCellGridDumpClearsEmptyCells(t *testing.T) {
	// Simulate a previous frame by pre-populating the writer with 'X'.
	w := term.NewStringWriter(10, 2)
	for y := range 2 {
		for x := range 10 {
			w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{Ch: 'X', Width: 1})
		}
	}

	// New frame: grid has sparse content — only "AB" on row 0.
	var g cellGrid
	g.Resize(10, 2)
	setGridText(&g, 0, "AB")
	// Row 1 is entirely empty (Ch=0 after Clear).

	g.Dump(w, nil)

	cells := w.Cells()
	// Row 0: A, B at x=0,1; remaining cells should be overwritten (not 'X').
	assert.Equal(t, 'A', cells[0].Ch)
	assert.Equal(t, 'B', cells[1].Ch)
	for x := 2; x < 10; x++ {
		assert.NotEqual(t, 'X', cells[x].Ch,
			"x=%d y=0 should be cleared, not retain old content", x)
	}
	// Row 1: entirely empty in grid — all cells should be overwritten.
	for x := range 10 {
		assert.NotEqual(t, 'X', cells[10+x].Ch,
			"x=%d y=1 should be cleared, not retain old content", x)
	}
}

func TestCellGridOutOfBounds(t *testing.T) {
	var g cellGrid
	g.Resize(5, 5)

	// These should not panic.
	g.SetCell(term.Coordinates{X: -1, Y: 0}, term.Cell{Ch: 'X'})
	g.SetCell(term.Coordinates{X: 0, Y: -1}, term.Cell{Ch: 'X'})
	g.SetCell(term.Coordinates{X: 5, Y: 0}, term.Cell{Ch: 'X'})
	g.SetCell(term.Coordinates{X: 0, Y: 5}, term.Cell{Ch: 'X'})
	g.UnionAttributes(term.Coordinates{X: -1, Y: 0}, term.Attributes{})
	g.UnionAttributes(term.Coordinates{X: 10, Y: 0}, term.Attributes{})
}
