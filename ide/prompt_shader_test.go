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

package ide

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/browser"
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

func newCells(rows, cols int, fill rune) [][]term.Cell {
	cells := make([][]term.Cell, rows)
	for y := range cells {
		cells[y] = make([]term.Cell, cols)
		for x := range cells[y] {
			cells[y][x].Ch = fill
		}
	}
	return cells
}

// fakeWindow is a minimal browser.Window used to drive
// dynamicVirtualShader in tests. Only Position/Width/Height/Closed
// are consulted by the shader; the remainder are zero stubs.
type fakeWindow struct {
	pos    term.Coordinates
	w, h   int
	closed bool
}

func (f *fakeWindow) Position() term.Coordinates               { return f.pos }
func (f *fakeWindow) Width() int                               { return f.w }
func (f *fakeWindow) Height() int                              { return f.h }
func (f *fakeWindow) Closed() bool                             { return f.closed }
func (f *fakeWindow) Content() (browserapi.Handler, error)     { return nil, nil }
func (f *fakeWindow) SetContent(browserapi.Handler) error      { return nil }
func (f *fakeWindow) Close() error                             { return nil }
func (f *fakeWindow) WindowID() uint64                         { return 1 }
func (f *fakeWindow) Focus() (bool, error)                     { return false, nil }
func (f *fakeWindow) IsFloating() bool                         { return true }
func (f *fakeWindow) IsMinimized() (component.Alignment, bool) { return 0, false }
func (f *fakeWindow) MinimizeUp(int) bool                      { return false }
func (f *fakeWindow) MinimizeDown(int) bool                    { return false }
func (f *fakeWindow) MinimizeLeft(int) bool                    { return false }
func (f *fakeWindow) MinimizeRight(int) bool                   { return false }
func (f *fakeWindow) Unminimize() bool                         { return false }
func (f *fakeWindow) SetFrameAttr(term.Attributes) (term.Attributes, bool) {
	return term.Attributes{}, false
}

var _ browser.Window = (*fakeWindow)(nil)

// TestDynamicVirtualTranslatesByOffset asserts that the window's
// position is combined with the supplied offset to locate the
// shaded sub-rectangle.
func TestDynamicVirtualTranslatesByOffset(t *testing.T) {
	t.Parallel()

	win := &fakeWindow{pos: term.Coordinates{X: 1, Y: 1}, w: 2, h: 2}
	rec := &recordingShader{mark: '#'}
	sh := dynamicVirtual(rec, win, term.Coordinates{X: 1, Y: 1})

	cells := newCells(5, 5, '.')
	sh.Shade(0, 1, cells)
	assert.Equal(t, 2, rec.gotRows)
	assert.Equal(t, 2, rec.gotCols)
	// pos(1,1) + offset(1,1) = (2,2), 2x2 region.
	for y := 2; y < 4; y++ {
		for x := 2; x < 4; x++ {
			assert.Equal(t, '#', cells[y][x].Ch,
				"cell (%d,%d) should be marked", x, y)
		}
	}
	assert.Equal(t, '.', cells[1][1].Ch, "outside region should be untouched")
	assert.Equal(t, '.', cells[4][4].Ch, "outside region should be untouched")
}

// TestDynamicVirtualClosedWindowIsNoOp asserts that the inner
// shader is not invoked when the window has been closed.
func TestDynamicVirtualClosedWindowIsNoOp(t *testing.T) {
	t.Parallel()
	win := &fakeWindow{pos: term.Coordinates{}, w: 2, h: 2, closed: true}
	rec := &recordingShader{mark: '#'}
	sh := dynamicVirtual(rec, win, term.Coordinates{})
	cells := newCells(2, 2, '.')
	sh.Shade(0, 1, cells)
	assert.Equal(t, 0, rec.gotRows)
	for y := range cells {
		for x := range cells[y] {
			assert.Equal(t, '.', cells[y][x].Ch)
		}
	}
}

// TestDynamicVirtualZeroDimsIsNoOp asserts that the inner shader is
// skipped while the window has not yet been laid out.
func TestDynamicVirtualZeroDimsIsNoOp(t *testing.T) {
	t.Parallel()
	win := &fakeWindow{}
	rec := &recordingShader{mark: '#'}
	sh := dynamicVirtual(rec, win, term.Coordinates{})
	cells := newCells(2, 2, '.')
	sh.Shade(0, 1, cells)
	assert.Equal(t, 0, rec.gotRows)
	for y := range cells {
		for x := range cells[y] {
			assert.Equal(t, '.', cells[y][x].Ch)
		}
	}
}

// TestDynamicVirtualClampsToMatrix asserts that an over-sized
// rectangle is truncated to the underlying matrix dimensions.
func TestDynamicVirtualClampsToMatrix(t *testing.T) {
	t.Parallel()
	win := &fakeWindow{pos: term.Coordinates{X: 1, Y: 1}, w: 10, h: 10}
	rec := &recordingShader{mark: '#'}
	sh := dynamicVirtual(rec, win, term.Coordinates{})
	cells := newCells(2, 2, '.')
	sh.Shade(0, 1, cells)
	assert.Equal(t, 1, rec.gotRows)
	assert.Equal(t, 1, rec.gotCols)
	assert.Equal(t, '#', cells[1][1].Ch)
	assert.Equal(t, '.', cells[0][0].Ch)
	assert.Equal(t, '.', cells[0][1].Ch)
	assert.Equal(t, '.', cells[1][0].Ch)
}
