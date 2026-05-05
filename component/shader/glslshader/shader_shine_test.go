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
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/component/shader/shadertest"
)

func TestShine(t *testing.T) {
	shadertest.TestShader(t, Shine(
		DefaultShineParams(),
		term.Attributes{Fg: tcell.NewRGBColor(80, 80, 80)},
	))
}

// TestShineSweep renders the entire sweep of the shine effect over a filled
// rectangle, one frame of animation per test case. Cells whose foreground was
// touched by the shine band render as '#' (via term.StringWriter.ForegroundCh);
// untouched cells render as the original 'x'.
func TestShineSweep(t *testing.T) {
	const (
		w     = 11
		h     = 5
		total = 8
	)

	mk := func(ch rune) term.Cell {
		// Untouched cells leave Fg as ColorDefault (zero) so the
		// StringWriter's ForegroundCh substitution only fires for cells
		// the band actually shaded.
		return term.Cell{Ch: ch, Width: 1}
	}

	makeCells := func() [][]term.Cell {
		cells := make([][]term.Cell, h)
		for y := range h {
			cells[y] = make([]term.Cell, w)
			for x := range w {
				cells[y][x] = mk('x')
			}
		}
		return cells
	}

	tsuite := []struct {
		name   string
		frame  int
		expect string
	}{
		{
			name:  "frame 0 of 8: band entering at bottom-left corner",
			frame: 0,
			expect: `
xxxxxxxxxxx
xxxxxxxxxxx
xxxxxxxxxxx
xxxxxxxxxxx
xxxxxxxxxxx`,
		},
		{
			name:  "frame 1 of 8",
			frame: 1,
			expect: `
xxxxxxxxxxx
xxxxxxxxxxx
xxxxxxxxxxx
##xxxxxxxxx
####xxxxxxx`,
		},
		{
			name:  "frame 2 of 8",
			frame: 2,
			expect: `
xxxxxxxxxxx
#xxxxxxxxxx
###xxxxxxxx
######xxxxx
########xxx`,
		},
		{
			name:  "frame 3 of 8",
			frame: 3,
			expect: `
###xxxxxxxx
#####xxxxxx
########xxx
##########x
x##########`,
		},
		{
			name:  "frame 4 of 8",
			frame: 4,
			expect: `
######xxxxx
#########xx
###########
xx#########
xxxxx######`,
		},
		{
			name:  "frame 5 of 8",
			frame: 5,
			expect: `
###########
x##########
xxxx#######
xxxxxx#####
xxxxxxxxx##`,
		},
		{
			name:  "frame 6 of 8",
			frame: 6,
			expect: `
xxx########
xxxxx######
xxxxxxxx###
xxxxxxxxxx#
xxxxxxxxxxx`,
		},
		{
			name:  "frame 7 of 8: band exiting at top-right corner",
			frame: 7,
			expect: `
xxxxxxx####
xxxxxxxxx##
xxxxxxxxxxx
xxxxxxxxxxx
xxxxxxxxxxx`,
		},
	}

	sh := Shine(
		ShineParams{
			Color:     tcell.NewRGBColor(255, 255, 255),
			BandWidth: 0.3,
			Cycles:    1,
		},
		// A non-default Fg fallback so the shine has something to blend
		// from on cells whose Fg is ColorDefault.
		term.Attributes{Fg: tcell.NewRGBColor(80, 80, 80)},
	)

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			cells := makeCells()
			sh.Shade(tcase.frame, total, cells)

			tw := term.NewStringWriter(w, h)
			tw.ForegroundCh = '#'
			for y, row := range cells {
				for x, cell := range row {
					tw.SetCell(term.Coordinates{X: x, Y: y}, cell)
				}
			}
			require.NoError(t, tw.Flush())
			assert.Equal(t, tcase.expect, "\n"+tw.String())
		})
	}
}

// TestShineDirections runs the shine sweep at the same frame index against
// each Direction, demonstrating the band's orientation. The grid below is
// the matrix the shader sees (11 cols x 5 rows). Untouched cells render as
// 'x'; touched cells as '#'.
func TestShineDirections(t *testing.T) {
	const (
		w     = 11
		h     = 5
		total = 8
		// Frame 2 of 8 puts the band near the start of the sweep so the
		// orientation reads clearly and opposite directions look mirrored
		// (frame 4 would land mid-screen, indistinguishable across opposite
		// pairs).
		frame = 2
	)

	mk := func(ch rune) term.Cell {
		return term.Cell{Ch: ch, Width: 1}
	}
	makeCells := func() [][]term.Cell {
		cells := make([][]term.Cell, h)
		for y := range h {
			cells[y] = make([]term.Cell, w)
			for x := range w {
				cells[y][x] = mk('x')
			}
		}
		return cells
	}

	tsuite := []struct {
		name      string
		direction Direction
		expect    string
	}{
		{
			name:      "bottom-left to top-right",
			direction: DirectionBottomLeftToTopRight,
			expect: `
xxxxxxxxxxx
#xxxxxxxxxx
###xxxxxxxx
######xxxxx
########xxx`,
		},
		{
			name:      "top-left to bottom-right",
			direction: DirectionTopLeftToBottomRight,
			expect: `
########xxx
######xxxxx
###xxxxxxxx
#xxxxxxxxxx
xxxxxxxxxxx`,
		},
		{
			name:      "left to right",
			direction: DirectionLeftToRight,
			expect: `
####xxxxxxx
####xxxxxxx
####xxxxxxx
####xxxxxxx
####xxxxxxx`,
		},
		{
			name:      "right to left",
			direction: DirectionRightToLeft,
			expect: `
xxxxxxx####
xxxxxxx####
xxxxxxx####
xxxxxxx####
xxxxxxx####`,
		},
		{
			name:      "top to bottom",
			direction: DirectionTopToBottom,
			expect: `
###########
###########
xxxxxxxxxxx
xxxxxxxxxxx
xxxxxxxxxxx`,
		},
		{
			name:      "bottom to top",
			direction: DirectionBottomToTop,
			expect: `
xxxxxxxxxxx
xxxxxxxxxxx
xxxxxxxxxxx
###########
###########`,
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			sh := Shine(
				ShineParams{
					Direction: tcase.direction,
					Color:     tcell.NewRGBColor(255, 255, 255),
					BandWidth: 0.3,
					Cycles:    1,
				},
				term.Attributes{Fg: tcell.NewRGBColor(80, 80, 80)},
			)

			cells := makeCells()
			sh.Shade(frame, total, cells)

			tw := term.NewStringWriter(w, h)
			tw.ForegroundCh = '#'
			for y, row := range cells {
				for x, cell := range row {
					tw.SetCell(term.Coordinates{X: x, Y: y}, cell)
				}
			}
			require.NoError(t, tw.Flush())
			assert.Equal(t, tcase.expect, "\n"+tw.String())
		})
	}
}
