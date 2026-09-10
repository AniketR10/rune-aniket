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

import "testing"

func TestMat2(t *testing.T) {
	tsuite := []struct {
		name               string
		a11, a21, a12, a22 float64
		want               matD2x2
	}{
		{"identity matrix", 1, 0, 0, 1, matD2x2{1, 0, 0, 1}},
		{"zero matrix", 0, 0, 0, 0, matD2x2{0, 0, 0, 0}},
		{"random matrix", 1, 2, 3, 4, matD2x2{1, 2, 3, 4}},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			if got := mat2(tcase.a11, tcase.a21, tcase.a12, tcase.a22); got != tcase.want {
				t.Errorf("mat2() = %v, want %v", got, tcase.want)
			}
		})
	}
}

func TestMultVec2D(t *testing.T) {
	tsuite := []struct {
		name string
		m    matD2x2
		v    vec2D
		want vec2D
	}{
		{"identity matrix with zero vector", matD2x2{1, 0, 0, 1}, vec2D{0, 0}, vec2D{0, 0}},
		{"identity matrix with unit vector", matD2x2{1, 0, 0, 1}, vec2D{1, 1}, vec2D{1, 1}},
		{"random matrix with random vector", matD2x2{1, 2, 3, 4}, vec2D{1, 2}, vec2D{7, 10}},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			if got := tcase.m.multVec2D(tcase.v); got != tcase.want {
				t.Errorf("multVec2D() = %v, want %v", got, tcase.want)
			}
		})
	}
}
