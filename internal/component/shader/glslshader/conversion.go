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

// nolint: unused
package glslshader

import (
	"github.com/unstablebuild/rune-go-sdk/term"
)

func colToVec(col, resolveColorDefault term.Color) vec3D {
	if col == term.ColorDefault {
		col = resolveColorDefault
	}
	r, g, b := col.RGB()
	return vec3(float(r), float(g), float(b))

}

func vecToCol(v vec3D) term.Color {
	return term.NewRGBColor(int32(v.x), int32(v.y), int32(v.z))
}
