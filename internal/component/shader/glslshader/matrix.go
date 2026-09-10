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

func mat2(a11, a21, a12, a22 float) matD2x2 {
	return matD2x2{a11, a21, a12, a22}
}

type matD2x2 struct {
	a11 float
	a21 float
	a12 float
	a22 float
}

func (m matD2x2) multVec2D(v vec2D) vec2D {
	return vec2D{
		x: m.a11*v.x + m.a12*v.y,
		y: m.a21*v.x + m.a22*v.y,
	}
}
