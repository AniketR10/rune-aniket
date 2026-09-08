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
	"unstable.build/rune/internal/component/shader/shaderutils"
)

// Fade is a Shader that interpolates the foreground and background color
// slowly as epoc progresses, creating a fade in effect.
func Fade(defaultAttrs term.Attributes) Shader {
	return &fade{defaultAttr: defaultAttrs}
}

type fade struct {
	defaultAttr term.Attributes
}

func (s fade) Shade(epoch, total int, cells [][]term.Cell) {
	if epoch >= total {
		return
	}
	if epoch == 0 {
		for y, row := range cells {
			for x, cell := range row {
				if cell.Bg == term.ColorDefault {
					cell.Bg = s.defaultAttr.Bg
				}
				cells[y][x].Fg = cell.Bg
			}
		}
		return
	}
	opacity := float64(epoch) / float64(total)
	for y, row := range cells {
		for x, cell := range row {
			if cell.Bg == term.ColorDefault {
				cell.Bg = s.defaultAttr.Bg
			}
			if cell.Fg == term.ColorDefault {
				cell.Fg = s.defaultAttr.Fg
			}
			cells[y][x].Fg = shaderutils.InterpolateColor(
				opacity, cell.Bg, cell.Fg, s.defaultAttr.Fg,
			)
		}
	}
}
