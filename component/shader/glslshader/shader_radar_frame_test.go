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

package glslshader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/component/shader/shadertest"
)

func TestRadarFrame(t *testing.T) {
	shadertest.TestShader(t, RadarFrame(
		DefaultRadarFrameParams(guiFrameCharset()),
		term.Attributes{Fg: term.NewRGBColor(80, 80, 80)},
	))
}

// TestRadarFrameLeavesNonFrameCharsUntouched scans the sweep through a full
// revolution against a grid of non-frame characters and verifies their
// foreground is never modified.
func TestRadarFrameLeavesNonFrameCharsUntouched(t *testing.T) {
	fc := guiFrameCharset()
	origFg := term.NewRGBColor(10, 20, 30)
	mk := func(ch rune) term.Cell {
		return term.Cell{
			Ch:         ch,
			Attributes: term.Attributes{Fg: origFg},
			Width:      1,
		}
	}

	cells := [][]term.Cell{
		{mk('x'), mk('y'), mk('z')},
		{mk('a'), mk('b'), mk('c')},
		{mk('1'), mk('2'), mk('3')},
	}

	sh := RadarFrame(
		DefaultRadarFrameParams(fc),
		term.Attributes{Fg: term.NewRGBColor(80, 80, 80)},
	)
	for f := range 12 {
		sh.Shade(f, 12, cells)
	}

	for _, row := range cells {
		for _, cell := range row {
			assert.Equal(t, origFg, cell.Fg)
		}
	}
}

// TestRadarFramePeakIsFullColor verifies that at the wedge peak, a frame
// cell along the leading angle is fully overridden to Color.
func TestRadarFramePeakIsFullColor(t *testing.T) {
	fc := guiFrameCharset()
	target := term.NewRGBColor(255, 0, 0)
	mk := func(ch rune) term.Cell {
		return term.Cell{
			Ch:         ch,
			Attributes: term.Attributes{Fg: term.NewRGBColor(10, 20, 30)},
			Width:      1,
		}
	}

	// Rectangular 11x5 frame. At frame 0 of 1 cycle the wedge points
	// along the positive-x axis (angle=0), so the right-middle vertical
	// edge cell sits exactly at the wedge peak.
	const (
		w = 11
		h = 5
	)
	cells := make([][]term.Cell, h)
	for y := range h {
		cells[y] = make([]term.Cell, w)
		for x := range w {
			switch {
			case y == 0 && x == 0:
				cells[y][x] = mk(fc.TopLeft)
			case y == 0 && x == w-1:
				cells[y][x] = mk(fc.TopRight)
			case y == h-1 && x == 0:
				cells[y][x] = mk(fc.BottomLeft)
			case y == h-1 && x == w-1:
				cells[y][x] = mk(fc.BottomRight)
			case y == 0:
				cells[y][x] = mk(fc.HorizontalTop)
			case y == h-1:
				cells[y][x] = mk(fc.HorizontalBottom)
			case x == 0:
				cells[y][x] = mk(fc.VerticalLeft)
			case x == w-1:
				cells[y][x] = mk(fc.VerticalRight)
			default:
				cells[y][x] = mk(' ')
			}
		}
	}

	params := DefaultRadarFrameParams(fc)
	params.Color = target
	params.AngularWidth = 0.5
	sh := RadarFrame(
		params,
		term.Attributes{Fg: term.NewRGBColor(80, 80, 80)},
	)
	sh.Shade(0, 1, cells)

	assert.Equal(t, target, cells[h/2][w-1].Fg,
		"right-middle frame cell sits at the wedge peak and must be fully overridden")
}

// TestRadarFrameFadesAtEdges verifies that the wedge edge is dimmer than
// the peak: a cell at the geometric center of the wedge is blended at
// higher intensity than a cell near the angular edge.
func TestRadarFrameFadesAtEdges(t *testing.T) {
	fc := guiFrameCharset()
	origFg := term.NewRGBColor(0, 0, 0)
	target := term.NewRGBColor(255, 255, 255)
	mk := func(ch rune) term.Cell {
		return term.Cell{
			Ch:         ch,
			Attributes: term.Attributes{Fg: origFg},
			Width:      1,
		}
	}

	// Tall narrow frame so the top and bottom edges are at clearly
	// distinct angles relative to the right-middle peak.
	const (
		w = 11
		h = 5
	)
	makeCells := func() [][]term.Cell {
		cells := make([][]term.Cell, h)
		for y := range h {
			cells[y] = make([]term.Cell, w)
			for x := range w {
				switch {
				case y == 0 && x == 0:
					cells[y][x] = mk(fc.TopLeft)
				case y == 0 && x == w-1:
					cells[y][x] = mk(fc.TopRight)
				case y == h-1 && x == 0:
					cells[y][x] = mk(fc.BottomLeft)
				case y == h-1 && x == w-1:
					cells[y][x] = mk(fc.BottomRight)
				case y == 0:
					cells[y][x] = mk(fc.HorizontalTop)
				case y == h-1:
					cells[y][x] = mk(fc.HorizontalBottom)
				case x == 0:
					cells[y][x] = mk(fc.VerticalLeft)
				case x == w-1:
					cells[y][x] = mk(fc.VerticalRight)
				default:
					cells[y][x] = mk(' ')
				}
			}
		}
		return cells
	}

	params := DefaultRadarFrameParams(fc)
	params.Color = target
	params.AngularWidth = 0.6
	sh := RadarFrame(
		params,
		term.Attributes{Fg: origFg},
	)

	cells := makeCells()
	sh.Shade(0, 1, cells)

	peakCell := cells[h/2][w-1]
	edgeCell := cells[0][w-1]

	peakR, _, _ := peakCell.Fg.RGB()
	edgeR, _, _ := edgeCell.Fg.RGB()

	assert.Greater(t, int(peakR), int(edgeR),
		"the peak of the wedge must be brighter than its angular edge")
	assert.Greater(t, int(edgeR), int(0),
		"the edge of the wedge must still be partially blended (not the original)")
}

// TestRadarFrameRotates verifies the wedge actually moves across frames:
// the same frame cell receives different intensities at different times.
func TestRadarFrameRotates(t *testing.T) {
	fc := guiFrameCharset()
	origFg := term.NewRGBColor(0, 0, 0)
	target := term.NewRGBColor(255, 255, 255)
	mk := func(ch rune) term.Cell {
		return term.Cell{
			Ch:         ch,
			Attributes: term.Attributes{Fg: origFg},
			Width:      1,
		}
	}
	makeCells := func() [][]term.Cell {
		return [][]term.Cell{
			{mk(fc.TopLeft), mk(fc.HorizontalTop), mk(fc.TopRight)},
			{mk(fc.VerticalLeft), mk(' '), mk(fc.VerticalRight)},
			{mk(fc.BottomLeft), mk(fc.HorizontalBottom), mk(fc.BottomRight)},
		}
	}

	params := DefaultRadarFrameParams(fc)
	params.Color = target
	params.AngularWidth = 0.25

	sh := RadarFrame(params, term.Attributes{Fg: origFg})

	// At frame 0 (angle 0, pointing right) the right-middle edge is at
	// the peak; at frame 2 of 4 (angle 0.5, pointing left) it is on the
	// far side and should be unaffected.
	cellsA := makeCells()
	sh.Shade(0, 4, cellsA)
	cellsB := makeCells()
	sh.Shade(2, 4, cellsB)

	assert.NotEqual(t, origFg, cellsA[1][2].Fg,
		"right-middle edge should be blended at frame 0 (wedge faces right)")
	assert.Equal(t, origFg, cellsB[1][2].Fg,
		"right-middle edge should be untouched at frame 2 (wedge faces left)")
}
