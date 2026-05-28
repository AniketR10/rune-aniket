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

package shader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
)

type recordingShader struct {
	gotRows, gotCols int
	mark             rune
}

func (s *recordingShader) Shade(_, _ int, cells [][]term.Cell) {
	s.gotRows = len(cells)
	for _, row := range cells {
		if len(row) > s.gotCols {
			s.gotCols = len(row)
		}
		for x := range row {
			row[x].Ch = s.mark
		}
	}
}

// TestVirtualClipsAndOffsets asserts that the inner shader sees only
// the sub-rectangle and that mutations land at the requested offset in
// the original matrix.
func TestVirtualClipsAndOffsets(t *testing.T) {
	t.Parallel()
	cells := make([][]term.Cell, 4)
	for y := range cells {
		cells[y] = make([]term.Cell, 6)
		for x := range cells[y] {
			cells[y][x].Ch = '.'
		}
	}

	rec := &recordingShader{mark: '#'}
	Virtual(rec, term.Coordinates{X: 2, Y: 1}, 3, 2).Shade(0, 1, cells)

	assert.Equal(t, 2, rec.gotRows,
		"inner shader should see Height rows")
	assert.Equal(t, 3, rec.gotCols,
		"inner shader should see Width cols")

	for y := 1; y < 3; y++ {
		for x := 2; x < 5; x++ {
			assert.Equal(t, '#', cells[y][x].Ch,
				"cell (%d, %d) inside sub-rect should be mutated", x, y)
		}
	}
	for y := range cells {
		for x := range cells[y] {
			if y >= 1 && y < 3 && x >= 2 && x < 5 {
				continue
			}
			assert.Equal(t, '.', cells[y][x].Ch,
				"cell (%d, %d) outside sub-rect must be untouched", x, y)
		}
	}
}

// TestVirtualOutOfRangeIsNoOp asserts that zero dimensions, negative
// offsets, and offsets past the bottom edge mutate no cells.
func TestVirtualOutOfRangeIsNoOp(t *testing.T) {
	t.Parallel()
	cells := [][]term.Cell{
		{{Ch: 'a'}, {Ch: 'b'}},
		{{Ch: 'c'}, {Ch: 'd'}},
	}
	clone := [][]term.Cell{
		{{Ch: 'a'}, {Ch: 'b'}},
		{{Ch: 'c'}, {Ch: 'd'}},
	}

	rec := &recordingShader{mark: '#'}
	for _, sh := range []Shader{
		Virtual(rec, term.Coordinates{X: 0, Y: 0}, 0, 0),
		Virtual(rec, term.Coordinates{X: -1, Y: 0}, 2, 2),
		Virtual(rec, term.Coordinates{X: 0, Y: 99}, 2, 2),
	} {
		sh.Shade(0, 1, cells)
	}
	assert.Equal(t, clone, cells,
		"out-of-range Virtual must not mutate any cell")
}

// TestVirtualClampsWidthHeightToMatrix asserts that Width/Height
// larger than the underlying matrix are truncated to what is available.
func TestVirtualClampsWidthHeightToMatrix(t *testing.T) {
	t.Parallel()
	cells := [][]term.Cell{
		{{Ch: 'a'}, {Ch: 'b'}},
		{{Ch: 'c'}, {Ch: 'd'}},
	}
	rec := &recordingShader{mark: '#'}
	Virtual(rec, term.Coordinates{X: 1, Y: 1}, 10, 10).Shade(0, 1, cells)
	assert.Equal(t, 1, rec.gotRows)
	assert.Equal(t, 1, rec.gotCols)
	assert.Equal(t, '#', cells[1][1].Ch)
	assert.Equal(t, 'a', cells[0][0].Ch)
	assert.Equal(t, 'b', cells[0][1].Ch)
	assert.Equal(t, 'c', cells[1][0].Ch)
}