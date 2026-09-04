// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package starlarktutorial

import (
	"math"
	"time"

	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/rune/component/shader"
	"unstable.build/rune/component/shader/shaderutils"
)

const hintFPS = 30
const hintDuration = 3 * time.Second

// defaultAttr.Fg resolves cells whose Fg is [term.ColorDefault];
// when both are default, the underlying [shaderutils.InterpolateColor]
// short-circuits to a no-op rather than emitting near-black RGB.
func buildHintPulse(defaultAttr term.Attributes, x, y, width int) shader.Shader {
	return shader.Virtual(
		&textBlink{
			target:       term.ColorGray,
			defaultFg:    defaultAttr.Fg,
			periodFrames: 30,
		},
		term.Coordinates{X: x, Y: y}, width, 1,
	)
}

type textBlink struct {
	target       term.Color
	defaultFg    term.Color
	periodFrames int
}

func (s *textBlink) Shade(frame, _ int, cells [][]term.Cell) {
	if s.periodFrames < 2 || frame < 0 {
		return
	}
	// Sine cycles 0..1..0 over periodFrames; smoother than a
	// triangle wave for a perceptual heartbeat.
	cyclePos := float64(frame%s.periodFrames) / float64(s.periodFrames)
	intensity := math.Sin(math.Pi * cyclePos)
	if intensity <= 0 {
		return
	}
	for y := range cells {
		for x := range cells[y] {
			ch := cells[y][x].Ch
			if ch == 0 || ch == ' ' {
				continue
			}
			cells[y][x].Fg = shaderutils.InterpolateColor(
				intensity, cells[y][x].Fg, s.target, s.defaultFg,
			)
		}
	}
}
