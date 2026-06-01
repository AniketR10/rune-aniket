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

func TestPulseFrame(t *testing.T) {
	shadertest.TestShader(t, PulseFrame(
		DefaultPulseFrameParams(guiFrameCharset()),
		term.Attributes{Fg: term.NewRGBColor(80, 80, 80)},
	))
}

// TestPulseFrameOnlyAffectsFrameChars verifies the pulse leaves every
// non-frame cell untouched while uniformly blending all frame cells.
func TestPulseFrameOnlyAffectsFrameChars(t *testing.T) {
	fc := guiFrameCharset()
	origFg := term.NewRGBColor(10, 20, 30)
	mk := func(ch rune) term.Cell {
		return term.Cell{
			Ch:         ch,
			Attributes: term.Attributes{Fg: origFg},
			Width:      1,
		}
	}

	// 3x3 with frame chars on the border and 'a' in the middle.
	makeCells := func() [][]term.Cell {
		return [][]term.Cell{
			{mk(fc.TopLeft), mk(fc.HorizontalTop), mk(fc.TopRight)},
			{mk(fc.VerticalLeft), mk('a'), mk(fc.VerticalRight)},
			{mk(fc.BottomLeft), mk(fc.HorizontalBottom), mk(fc.BottomRight)},
		}
	}

	sh := PulseFrame(
		DefaultPulseFrameParams(fc),
		term.Attributes{Fg: term.NewRGBColor(80, 80, 80)},
	)

	// At the trough (frame 0) the foreground is unchanged; at the peak
	// (mid-animation) every frame cell shares the same blended color
	// while the inner 'a' is left untouched.
	const total = 10
	cells := makeCells()
	sh.Shade(0, total, cells)
	for _, row := range cells {
		for _, cell := range row {
			assert.Equal(t, origFg, cell.Fg,
				"trough: cell %q must be untouched", string(cell.Ch))
		}
	}

	cells = makeCells()
	sh.Shade(total/2, total, cells)
	var peakFrameFg term.Color
	first := true
	for y, row := range cells {
		for x, cell := range row {
			if y == 1 && x == 1 {
				assert.Equal(t, origFg, cell.Fg,
					"peak: inner non-frame cell must be untouched")
				continue
			}
			if first {
				peakFrameFg = cell.Fg
				first = false
				assert.NotEqual(t, origFg, peakFrameFg,
					"peak: frame cells should blend away from original")
				continue
			}
			assert.Equal(t, peakFrameFg, cell.Fg,
				"peak: every frame cell shares the same color")
		}
	}
}

// TestPulseFrameMinIntensityClampsTrough verifies MinIntensity keeps the
// pulse blended even at the bottom of the cycle.
func TestPulseFrameMinIntensityClampsTrough(t *testing.T) {
	fc := guiFrameCharset()
	origFg := term.NewRGBColor(10, 20, 30)
	cells := [][]term.Cell{{{
		Ch:         fc.TopLeft,
		Attributes: term.Attributes{Fg: origFg},
		Width:      1,
	}}}
	params := DefaultPulseFrameParams(fc)
	params.MinIntensity = 0.5
	sh := PulseFrame(params, term.Attributes{Fg: term.NewRGBColor(80, 80, 80)})

	sh.Shade(0, 10, cells)
	assert.NotEqual(t, origFg, cells[0][0].Fg,
		"trough with MinIntensity > 0 should still blend the color")
}
