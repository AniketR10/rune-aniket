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

package gui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// TestOffsetGapBackground covers the case that exposed the bug: a status
// bar whose cells carry a vertical render offset, under a shader that
// gives every cell its own background. The strip each cell vacates must
// follow the column it sits in, not one colour for the whole bar, or the
// bar cuts a flat band across the shader.
func TestOffsetGapBackground(t *testing.T) {
	// Distinct per-column backgrounds stand in for a shader gradient.
	shaded := func(y, x int, attrs term.AttrMask) term.Cell {
		return term.Cell{Ch: 'x', Attrs: attrs,
			Bg: term.NewColor(int32(10*y+x), 0, 0)}
	}
	grid := func(offsetRow int, attrs term.AttrMask) [][]term.Cell {
		cells := make([][]term.Cell, 3)
		for y := range cells {
			cells[y] = make([]term.Cell, 3)
			for x := range cells[y] {
				var cellAttrs term.AttrMask
				if y == offsetRow {
					cellAttrs = attrs
				}
				cells[y][x] = shaded(y, x, cellAttrs)
			}
		}
		return cells
	}

	tests := []struct {
		name  string
		cells [][]term.Cell
		viewY int
		x     int
		want  term.Color
	}{{
		name:  "a cell shifted down takes the column above it",
		cells: grid(1, term.AttrVerticalRenderOffset),
		viewY: 1, x: 2,
		want: term.NewColor(2, 0, 0),
	}, {
		name:  "a cell shifted up takes the column below it",
		cells: grid(1, term.AttrNegativeVerticalRenderOffset),
		viewY: 1, x: 2,
		want: term.NewColor(22, 0, 0),
	}, {
		name:  "shifted down at the frame top keeps its own background",
		cells: grid(0, term.AttrVerticalRenderOffset),
		viewY: 0, x: 1,
		want: term.NewColor(1, 0, 0),
	}, {
		name:  "shifted up at the frame bottom keeps its own background",
		cells: grid(2, term.AttrNegativeVerticalRenderOffset),
		viewY: 2, x: 1,
		want: term.NewColor(21, 0, 0),
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want,
				offsetGapBackground(tc.cells, tc.viewY, tc.x))
		})
	}
}

func TestOffsetGapStrip(t *testing.T) {
	const height = 20
	tests := []struct {
		name                string
		rowPixelY           float64
		cellPixelY          float64
		wantTop, wantBottom float64
	}{{
		name:      "a cell shifted down vacates the top of its strip",
		rowPixelY: 100, cellPixelY: 108,
		wantTop: 100, wantBottom: 108,
	}, {
		name:      "a cell shifted up vacates the bottom of its strip",
		rowPixelY: 100, cellPixelY: 92,
		wantTop: 112, wantBottom: 120,
	}, {
		name:      "an offset clamped to the frame top vacates nothing",
		rowPixelY: 0, cellPixelY: 0,
		wantTop: 0, wantBottom: 0,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			top, bottom := offsetGapStrip(tc.rowPixelY, tc.cellPixelY, height)
			assert.Equal(t, tc.wantTop, top)
			assert.Equal(t, tc.wantBottom, bottom)
		})
	}
}

// TestFillOffsetGapOnlyWhenVisible asserts the gap fill costs nothing
// while the row background is the default one (the frame fill already
// covers the strip) and emits geometry once a shader tints it.
func TestFillOffsetGapOnlyWhenVisible(t *testing.T) {
	r, _, _ := newTestRenderer(t, 8, 4)
	rowPixelY := r.fontManager.PixelY(2)

	r.fillOffsetGap(0, 1, rowPixelY, rowPixelY+4, term.ColorDefault)
	assert.True(t, r.rectBatch.Empty(), "default background needs no fill")

	r.fillOffsetGap(0, 1, rowPixelY, rowPixelY, term.NewColor(132, 132, 132))
	assert.True(t, r.rectBatch.Empty(), "an empty strip needs no fill")

	r.fillOffsetGap(0, 1, rowPixelY, rowPixelY+4, term.NewColor(132, 132, 132))
	assert.False(t, r.rectBatch.Empty(), "a tinted row fills the vacated strip")
}

// TestRenderRowOffsetOverDefaultEmitsNothing asserts a chrome row that
// is offset over the default background stays free of geometry: the
// frame fill already covers the strip the offset vacates, so no shader
// means no extra rects.
func TestRenderRowOffsetOverDefaultEmitsNothing(t *testing.T) {
	r, cols, rows := newTestRenderer(t, 8, 4)
	cells := filledGrid(rows, cols, 'x')
	for x := range cells[1] {
		cells[1][x].Attrs = term.AttrVerticalRenderOffset
	}
	r.renderRow(nil, cells, 1, passRects)
	assert.True(t, r.rectBatch.Empty())
}
