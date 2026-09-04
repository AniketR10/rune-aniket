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

package glslshader

import (
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/component/shader"
	"unstable.build/rune/component/shader/shaderutils"
)

// PulseFrame produces a temporal pulsing effect on every cell whose character
// belongs to the configured [component.FrameCharSet]: the whole frame breathes
// in unison towards [PulseFrameParams.Color] and back to its original
// foreground.
//
// Unlike [ShineFrame] there is no spatial direction; at any given frame all
// matching cells share the same blend intensity. Use this when you want to
// emphasize the entire frame rather than draw a sweeping highlight across it.
func PulseFrame(params PulseFrameParams, defaultAttr term.Attributes) shader.Shader {
	return &pulseFrame{PulseFrameParams: params, defaultAttr: defaultAttr}
}

// PulseFrameParams allows you to customize the [PulseFrame] effect.
type PulseFrameParams struct {
	// FrameCharSet is the set of frame characters the pulse effect is
	// applied to. Cells whose character is not part of this set are left
	// untouched.
	FrameCharSet component.FrameCharSet
	// Color is the color blended into the foreground of matching cells at
	// the peak of each pulse.
	Color term.Color
	// MinIntensity is the residual blend factor at the trough of the pulse,
	// in the [0, 1] range. 0 returns the frame to its original color; values
	// closer to 1 keep more of [Color] visible between peaks.
	MinIntensity float
	// Cycles is the number of full pulse cycles played across the animation.
	// Values less than 1 are treated as 1.
	Cycles int
}

// DefaultPulseFrameParams returns a sane set of [PulseFrameParams] using the
// given [component.FrameCharSet] as the set of cells the pulse applies to.
func DefaultPulseFrameParams(fc component.FrameCharSet) PulseFrameParams {
	return PulseFrameParams{
		FrameCharSet: fc,
		Color:        term.NewRGBColor(255, 255, 255),
		MinIntensity: 0.0,
		Cycles:       1,
	}
}

type pulseFrame struct {
	PulseFrameParams
	defaultAttr term.Attributes
}

func (s *pulseFrame) Shade(frame, total int, in [][]term.Cell) {
	if total <= 0 || frame < 0 || frame >= total {
		return
	}
	if len(in) == 0 {
		return
	}

	cycles := s.Cycles
	if cycles < 1 {
		cycles = 1
	}
	minI := clamp(s.MinIntensity, 0.0, 1.0)

	// triangleWave maps phase in [0, 1) to a [0, 1, 0] ramp so the pulse
	// rises and falls symmetrically without needing math.Sin.
	phase := fract(float(frame) / float(total) * float(cycles))
	tri := 1.0 - abs(2.0*phase-1.0)
	intensity := minI + (1.0-minI)*smoothstep(0.0, 1.0, tri)
	if intensity <= 0 {
		return
	}

	for y, row := range in {
		for x, cell := range row {
			if !s.isFrameChar(cell.Ch) {
				continue
			}
			in[y][x].Fg = shaderutils.InterpolateColor(
				intensity, cell.Fg, s.Color, s.defaultAttr.Fg,
			)
		}
	}
}

func (s *pulseFrame) isFrameChar(ch rune) bool {
	if ch == 0 {
		return false
	}
	fc := s.FrameCharSet
	return ch == fc.HorizontalTop ||
		ch == fc.HorizontalBottom ||
		ch == fc.VerticalLeft ||
		ch == fc.VerticalRight ||
		ch == fc.TopLeft ||
		ch == fc.TopRight ||
		ch == fc.BottomLeft ||
		ch == fc.BottomRight
}
