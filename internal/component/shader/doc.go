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

/*
Package shader contains all you need to run effects on the characters,
foreground and background of Ox's TUI/GUI.

A shader takes a frame, the total number of frames and a [term.Cell] matrix and
simply writes to Ch (character), Fg (foreground color) or Bg (background color)
of that cell.

	func MyShader() shader.Shader {
		return &myShader{}
	}

	type myShader struct {}

	func (s *myShader) Shade(frame, total int, cells [][]term.Cell) {
		for y, row := range cells {
			for x, cell := range row {
				cells[y][x].Ch = ...
				cells[y][x].Fg = ...
				cells[y][x].Fg = ...
			}
		}
	}

If you want to do more complex visuals with maths like the shaders you see in
[Shadertoy] have a look at the [Writing Pixel Shaders Tutorial].

[Shadertoy]: https://www.shadertoy.com/
[Writing Pixel Shaders Tutorial]: https://x.unstable.build/docs/tutorials/ox/pixel_shader
*/
package shader
