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
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/component/shader"
)

// ProgressVizABCD shows the process percentage as the first 10 letters of the
// alphabet, A being the start and I the end of the animation duration. The
// background is painted green where x is highest and blue where y is highest.
func ProgressVizABCD() shader.Shader {
	return &progressVizABCD{}
}

type progressVizABCD struct{}

func (s *progressVizABCD) Shade(frame, total int, cells [][]term.Cell) {
	h := len(cells)
	w := len(cells[0])

	progress := float(frame) / float(total) // 0.0..1.0

	for y := range h {
		for x := range w {
			if cells[y] == nil || x >= len(cells[y]) {
				continue
			}

			uv := vec2(255.0*float(x)/float(w), 255.0*float(y)/float(h))
			cells[y][x].Bg = vecToCol(vec3(0.0, uv.x, uv.y))
			cells[y][x].Fg = vecToCol(vec3(0.0, 1.0-uv.x, 1.0-uv.y))
			cells[y][x].Ch = rune('A' + int(10.0*progress))
		}
	}
}
