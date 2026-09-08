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

package shadertest

import (
	"math"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// testShader1234 draws characters progressively from 0-9 based on progress
// frame/total; useful to assert against in your shadertest tests.
type testShader1234 struct{}

func (s *testShader1234) Shade(frame, total int, cells [][]term.Cell) {
	h := len(cells)
	w := len(cells[0])

	progress := int(math.Round(9.0 * float64(frame) / float64(total))) // 0..9

	for y := range h {
		for x := range w {
			if cells[y] == nil || x >= len(cells[y]) {
				continue
			}
			cells[y][x].Ch = rune('0' + progress)
		}
	}
}

// testShaderABCD draws characters progressively from A-J based on progress
// frame/total; useful to assert against in your shadertest tests.
type testShaderABCD struct{}

func (s *testShaderABCD) Shade(frame, total int, cells [][]term.Cell) {
	h := len(cells)
	w := len(cells[0])

	progress := int(math.Round(9.0 * float64(frame) / float64(total))) // A..I

	for y := range h {
		for x := range w {
			if cells[y] == nil || x >= len(cells[y]) {
				continue
			}
			cells[y][x].Ch = rune('A' + progress)
		}
	}
}
