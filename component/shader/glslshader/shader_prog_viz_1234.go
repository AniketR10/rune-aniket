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
	"math"

	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/component/shader"
)

// ProgressViz1234 shows the process percentage as numbers 0-9, 0 being the
// start and 9 the end of the animation duration. The background is painted red
// where x is highest and green where y is highest.
func ProgressViz1234() shader.Shader {
	return &progressViz1234{}
}

type progressViz1234 struct{}

func (s *progressViz1234) Shade(frame, total int, cells [][]term.Cell) {
	h := len(cells)
	w := len(cells[0])

	progress := int(math.Round(9.0 * float(frame) / float(total))) // 0..9

	for y := range h {
		for x := range w {
			if cells[y] == nil || x >= len(cells[y]) {
				continue
			}

			uv := vec2(255.0*float(x)/float(w), 255.0*float(y)/float(h))
			cells[y][x].Bg = vecToCol(vec3(uv.x, uv.y, 0.0))
			cells[y][x].Fg = vecToCol(vec3(1.0-uv.x, 1.0-uv.y, 0.0))
			cells[y][x].Ch = rune('0' + progress)
		}
	}
}
