// Copyright (C) 2017-2026 The Rune Authors
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
	"unstable.build/rune/internal/component/asciiart"
	"unstable.build/rune/internal/component/shader"
	"unstable.build/rune/internal/component/shader/shaderutils"
)

// ShineFrame produces a diagonal shining effect that sweeps from the bottom
// left of the screen to the top right.
//
// The effect is applied exclusively to cells whose character matches one of
// the runes in the configured [component.FrameCharSet]; every other cell is
// left untouched. As the band moves over a matching cell the cell's
// foreground color is interpolated towards [ShineFrameParams.Color] giving a
// glint impression along the frame strokes.
func ShineFrame(params ShineFrameParams, defaultAttr term.Attributes) shader.Shader {
	return &shineFrame{ShineFrameParams: params, defaultAttr: defaultAttr}
}

// ShineFrameParams allows you to customize the [ShineFrame] effect.
type ShineFrameParams struct {
	// Direction along which the shine band sweeps.
	Direction Direction
	// FrameCharSet is the set of frame characters the shine effect is
	// applied to. Cells whose character is not part of this set are left
	// untouched.
	FrameCharSet component.FrameCharSet
	// Color is the color blended into the foreground of matching cells when
	// the shine band passes over them.
	Color term.Color
	// BandWidth controls how wide the shine band is, expressed as a fraction
	// of the (aspect-ratio corrected) diagonal length.
	//
	// (range 0..1 clamped, must be > 0)
	BandWidth float
	// Cycles is the number of times the shine band sweeps across the animation.
	// Values less than 1 are treated as 1.
	Cycles int
}

// DefaultShineFrameParams returns a sane set of [ShineFrameParams] using the
// given [component.FrameCharSet] as the set of cells the shine applies to.
func DefaultShineFrameParams(fc component.FrameCharSet) ShineFrameParams {
	return ShineFrameParams{
		Direction:    DirectionBottomLeftToTopRight,
		FrameCharSet: fc,
		Color:        term.NewRGBColor(255, 255, 255),
		BandWidth:    0.25,
		Cycles:       1,
	}
}

type shineFrame struct {
	ShineFrameParams
	defaultAttr term.Attributes
}

func (s *shineFrame) Shade(frame, total int, in [][]term.Cell) {
	if total <= 0 || frame < 0 || frame >= total {
		return
	}
	rows := len(in)
	if rows == 0 {
		return
	}

	cols := 0
	for _, row := range in {
		if len(row) > cols {
			cols = len(row)
		}
	}
	if cols == 0 {
		return
	}

	cycles := s.Cycles
	if cycles < 1 {
		cycles = 1
	}
	bandWidth := clamp(s.BandWidth, 0.0, 1.0)
	if bandWidth <= 0 {
		return
	}

	// Sweep "pulse" from -bandWidth to 1+bandWidth so the band fully enters
	// from the bottom-left corner and fully exits past the top-right corner.
	//
	// The product is rounded explicitly: arm64 fuses it with the
	// subtraction into an FMA and keeps the extra precision, which lands
	// the band edge on a different cell than amd64 does.
	pulse := fract(float(frame) / float(total) * float(cycles))
	pos := float(pulse*(1.0+2.0*bandWidth)) - bandWidth

	maxX := float(cols - 1)
	if maxX <= 0 {
		maxX = 1
	}
	maxY := float(rows-1) * asciiart.HeightToWidthCellAspectRatio
	if maxY <= 0 {
		maxY = 1
	}

	for y, row := range in {
		// y axis is flipped (terminal y=0 at top) and scaled by the cell
		// aspect ratio so the diagonal looks visually balanced.
		yNorm := (float(rows-1-y) * asciiart.HeightToWidthCellAspectRatio) / maxY
		for x, cell := range row {
			if !s.isFrameChar(cell.Ch) {
				continue
			}
			xNorm := float(x) / maxX
			t := directionT(s.Direction, xNorm, yNorm)
			intensity := smoothstep(bandWidth, 0.0, abs(t-pos))
			if intensity <= 0 {
				continue
			}
			in[y][x].Fg = shaderutils.InterpolateColor(
				intensity, cell.Fg, s.Color, s.defaultAttr.Fg,
			)
		}
	}
}

func (s *shineFrame) isFrameChar(ch rune) bool {
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
