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
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/component/shader/shadertest"
)

func guiFrameCharset() component.FrameCharSet {
	return component.FrameCharSet{
		HorizontalTop:    '▔',
		HorizontalBottom: '▁',
		VerticalRight:    '▕',
		VerticalLeft:     '▏',
		TopLeft:          '🭽',
		TopRight:         '🭾',
		BottomLeft:       '🭼',
		BottomRight:      '🭿',
	}
}

func TestShineFrame(t *testing.T) {
	shadertest.TestShader(t, ShineFrame(
		DefaultShineFrameParams(guiFrameCharset()),
		term.Attributes{Fg: term.NewRGBColor(80, 80, 80)},
	))
}

// TestShineFrameOnlyAffectsFrameChars renders the entire sweep of the shine
// effect over a single rectangular frame, one frame of animation per test
// case. Cells whose foreground was touched by the shine band render as '#'
// (via term.StringWriter.ForegroundCh); untouched frame characters and the
// inner 'a' render as themselves.
func TestShineFrameOnlyAffectsFrameChars(t *testing.T) {
	fc := guiFrameCharset()

	const (
		w     = 11
		h     = 5
		total = 8
	)

	// mk returns a cell with default Fg/Bg so that StringWriter.ForegroundCh
	// only shows up on cells the shader actually touched.
	mk := func(ch rune) term.Cell {
		return term.Cell{Ch: ch, Width: 1}
	}

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
				case x == w/2 && y == h/2:
					cells[y][x] = mk('a')
				default:
					cells[y][x] = mk(' ')
				}
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
🭽▔▔▔▔▔▔▔▔▔🭾
▏         ▕
▏    a    ▕
▏         ▕
🭼▁▁▁▁▁▁▁▁▁🭿`,
		},
		{
			name:  "frame 1 of 8",
			frame: 1,
			expect: `
🭽▔▔▔▔▔▔▔▔▔🭾
▏         ▕
▏    a    ▕
#         ▕
####▁▁▁▁▁▁🭿`,
		},
		{
			name:  "frame 2 of 8",
			frame: 2,
			expect: `
🭽▔▔▔▔▔▔▔▔▔🭾
#         ▕
#    a    ▕
#         ▕
########▁▁🭿`,
		},
		{
			name:  "frame 3 of 8",
			frame: 3,
			expect: `
###▔▔▔▔▔▔▔🭾
#         ▕
#    a    ▕
#         ▕
🭼##########`,
		},
		{
			name:  "frame 4 of 8",
			frame: 4,
			expect: `
######▔▔▔▔🭾
#         ▕
#    a    #
▏         #
🭼▁▁▁▁######`,
		},
		{
			name:  "frame 5 of 8",
			frame: 5,
			expect: `
###########
▏         #
▏    a    #
▏         #
🭼▁▁▁▁▁▁▁▁##`,
		},
		{
			name:  "frame 6 of 8",
			frame: 6,
			expect: `
🭽▔▔########
▏         #
▏    a    #
▏         #
🭼▁▁▁▁▁▁▁▁▁🭿`,
		},
		{
			name:  "frame 7 of 8: band exiting at top-right corner",
			frame: 7,
			expect: `
🭽▔▔▔▔▔▔####
▏         #
▏    a    ▕
▏         ▕
🭼▁▁▁▁▁▁▁▁▁🭿`,
		},
	}

	sh := ShineFrame(
		ShineFrameParams{
			FrameCharSet: fc,
			Color:        term.NewRGBColor(255, 255, 255),
			BandWidth:    0.3,
			Cycles:       1,
		},
		term.Attributes{Fg: term.NewRGBColor(80, 80, 80)},
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

func TestShineFrameNoMatchingChars(t *testing.T) {
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
	}

	sh := ShineFrame(
		DefaultShineFrameParams(fc),
		term.Attributes{Fg: term.NewRGBColor(80, 80, 80)},
	)
	for f := range 10 {
		sh.Shade(f, 10, cells)
	}

	for _, row := range cells {
		for _, cell := range row {
			assert.Equal(t, origFg, cell.Fg)
		}
	}
}

// TestShineFrameDirections runs the frame shine sweep at the same frame
// against each Direction. The matrix is a single rectangular frame; the
// shine only applies to the frame characters and otherwise leaves the
// inner cells untouched. Untouched cells render as the original frame
// glyph (or blanks); touched cells render as '#'.
func TestShineFrameDirections(t *testing.T) {
	fc := guiFrameCharset()

	const (
		w     = 11
		h     = 5
		total = 8
		// Frame 2 of 8 places the band near the start of the sweep so the
		// orientation reads clearly per direction.
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

	tsuite := []struct {
		name      string
		direction Direction
		expect    string
	}{
		{
			name:      "bottom-left to top-right",
			direction: DirectionBottomLeftToTopRight,
			expect: `
🭽▔▔▔▔▔▔▔▔▔🭾
#         ▕
#         ▕
#         ▕
########▁▁🭿`,
		},
		{
			name:      "top-left to bottom-right",
			direction: DirectionTopLeftToBottomRight,
			expect: `
########▔▔🭾
#         ▕
#         ▕
#         ▕
🭼▁▁▁▁▁▁▁▁▁🭿`,
		},
		{
			name:      "left to right",
			direction: DirectionLeftToRight,
			expect: `
####▔▔▔▔▔▔🭾
#         ▕
#         ▕
#         ▕
####▁▁▁▁▁▁🭿`,
		},
		{
			name:      "right to left",
			direction: DirectionRightToLeft,
			expect: `
🭽▔▔▔▔▔▔####
▏         #
▏         #
▏         #
🭼▁▁▁▁▁▁####`,
		},
		{
			name:      "top to bottom",
			direction: DirectionTopToBottom,
			expect: `
###########
#         #
▏         ▕
▏         ▕
🭼▁▁▁▁▁▁▁▁▁🭿`,
		},
		{
			name:      "bottom to top",
			direction: DirectionBottomToTop,
			expect: `
🭽▔▔▔▔▔▔▔▔▔🭾
▏         ▕
▏         ▕
#         #
###########`,
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			sh := ShineFrame(
				ShineFrameParams{
					Direction:    tcase.direction,
					FrameCharSet: fc,
					Color:        term.NewRGBColor(255, 255, 255),
					BandWidth:    0.3,
					Cycles:       1,
				},
				term.Attributes{Fg: term.NewRGBColor(80, 80, 80)},
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
