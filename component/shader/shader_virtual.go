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

package shader

import (
	"github.com/unstablebuild/rune-go-sdk/term"
)

// Virtual returns a Shader that forwards only the cells inside
// (offset.X, offset.Y, width, height) to inner, presented as a
// (height x width) matrix with origin re-based to (0, 0). Cells
// outside the rectangle pass through unchanged. Negative offsets
// or zero-sized rectangles are no-ops.
func Virtual(inner Shader, offset term.Coordinates, width, height int) Shader {
	return &virtualShader{
		inner:  inner,
		offset: offset,
		width:  width,
		height: height,
	}
}

type virtualShader struct {
	inner  Shader
	offset term.Coordinates
	width  int
	height int
}

func (s *virtualShader) Shade(frame, total int, cells [][]term.Cell) {
	if s.width <= 0 || s.height <= 0 {
		return
	}
	if s.offset.Y >= len(cells) || s.offset.X < 0 || s.offset.Y < 0 {
		return
	}
	maxY := min(s.offset.Y+s.height, len(cells))
	if maxY <= s.offset.Y {
		return
	}
	view := make([][]term.Cell, 0, maxY-s.offset.Y)
	for y := s.offset.Y; y < maxY; y++ {
		row := cells[y]
		if s.offset.X >= len(row) {
			view = append(view, nil)
			continue
		}
		end := min(s.offset.X+s.width, len(row))
		view = append(view, row[s.offset.X:end])
	}
	s.inner.Shade(frame, total, view)
}
