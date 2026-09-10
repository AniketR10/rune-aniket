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
	"fmt"
	"math"
)

type vec2D struct {
	x, y float
}

func vec2(val1, val2 float) (v vec2D) {
	v.x = val1
	v.y = val2
	return
}

func vec2FromScalar(val1 float) (v vec2D) {
	v.x = val1
	v.y = val1
	return
}

func (v1 vec2D) add(v2 vec2D) (v vec2D) {
	v.x = v1.x + v2.x
	v.y = v1.y + v2.y
	return
}

func (v1 vec2D) addSc(sc float) (v vec2D) {
	v.x = v1.x + sc
	v.y = v1.y + sc
	return
}

func (v1 vec2D) sub(v2 vec2D) (v vec2D) {
	v.x = v1.x - v2.x
	v.y = v1.y - v2.y
	return
}

func (v1 vec2D) subSc(sc float) (v vec2D) {
	v.x = v1.x - sc
	v.y = v1.y - sc
	return
}

func (v1 vec2D) mult(v2 vec2D) (v vec2D) {
	v.x = v1.x * v2.x
	v.y = v1.y * v2.y
	return
}

func (v1 vec2D) multSc(sc float) (v vec2D) {
	v.x = v1.x * sc
	v.y = v1.y * sc
	return
}

func (v1 vec2D) div(v2 vec2D) (v vec2D) {
	v.x = v1.x / v2.x
	v.y = v1.y / v2.y
	return
}

func (v1 vec2D) divSc(sc float) (v vec2D) {
	v.x = v1.x / sc
	v.y = v1.y / sc
	return
}

func (v1 vec2D) floor() (v vec2D) {
	v.x = math.Floor(v1.x)
	v.y = math.Floor(v1.y)
	return
}

type vec3D struct {
	x, y, z float
}

func vec3(val1, val2, val3 float) (v vec3D) {
	v.x = val1
	v.y = val2
	v.z = val3
	return
}

func vec3FromScalar(val1 float) (v vec3D) {
	v.x = val1
	v.y = val1
	v.z = val1
	return
}

func (v1 vec3D) add(v2 vec3D) (v vec3D) {
	v.x = v1.x + v2.x
	v.y = v1.y + v2.y
	v.z = v1.z + v2.z
	return
}

func (v1 vec3D) addSc(sc float) (v vec3D) {
	v.x = v1.x + sc
	v.y = v1.y + sc
	v.z = v1.z + sc
	return
}

func (v1 vec3D) sub(v2 vec3D) (v vec3D) {
	v.x = v1.x - v2.x
	v.y = v1.y - v2.y
	v.z = v1.z - v2.z
	return
}

func (v1 vec3D) subSc(sc float) (v vec3D) {
	v.x = v1.x - sc
	v.y = v1.y - sc
	v.z = v1.z - sc
	return
}

func (v1 vec3D) mult(v2 vec3D) (v vec3D) {
	v.x = v1.x * v2.x
	v.y = v1.y * v2.y
	v.z = v1.z * v2.z
	return
}

func (v1 vec3D) multSc(sc float) (v vec3D) {
	v.x = v1.x * sc
	v.y = v1.y * sc
	v.z = v1.z * sc
	return
}

func (v1 vec3D) div(v2 vec3D) (v vec3D) {
	v.x = v1.x / v2.x
	v.y = v1.y / v2.y
	v.z = v1.z / v2.z
	return
}

func (v1 vec3D) divSc(sc float) (v vec3D) {
	v.x = v1.x / sc
	v.y = v1.y / sc
	v.z = v1.z / sc
	return
}

func (v1 vec3D) floor() (v vec3D) {
	v.x = math.Floor(v1.x)
	v.y = math.Floor(v1.y)
	v.z = math.Floor(v1.z)
	return
}

type vec4D struct {
	x, y, z, w float
}

func vec4(vals ...float) (v vec4D) {
	if len(vals) == 1 {
		vals = append(vals, vals[0])
		vals = append(vals, vals[0])
		vals = append(vals, vals[0])
	}
	if len(vals) != 4 {
		panic(fmt.Sprintf(
			"incorrent number of components (%d) for 4D vector", len(vals),
		))
	}
	v.x = vals[0]
	v.y = vals[1]
	v.z = vals[2]
	v.w = vals[3]
	return
}

func (v1 vec4D) add(v2 vec4D) (v vec4D) {
	v.x = v1.x + v2.x
	v.y = v1.y + v2.y
	v.z = v1.z + v2.z
	v.w = v1.w + v2.w
	return
}

func (v1 vec4D) addSc(sc float) (v vec4D) {
	v.x = v1.x + sc
	v.y = v1.y + sc
	v.z = v1.z + sc
	v.w = v1.w + sc
	return
}

func (v1 vec4D) sub(v2 vec4D) (v vec4D) {
	v.x = v1.x - v2.x
	v.y = v1.y - v2.y
	v.z = v1.z - v2.z
	v.w = v1.w - v2.w
	return
}

func (v1 vec4D) subSc(sc float) (v vec4D) {
	v.x = v1.x - sc
	v.y = v1.y - sc
	v.z = v1.z - sc
	v.w = v1.w - sc
	return
}

func (v1 vec4D) mult(v2 vec4D) (v vec4D) {
	v.x = v1.x * v2.x
	v.y = v1.y * v2.y
	v.z = v1.z * v2.z
	v.w = v1.w * v2.w
	return
}

func (v1 vec4D) multSc(sc float) (v vec4D) {
	v.x = v1.x * sc
	v.y = v1.y * sc
	v.z = v1.z * sc
	v.w = v1.w * sc
	return
}

func (v1 vec4D) div(v2 vec4D) (v vec4D) {
	v.x = v1.x / v2.x
	v.y = v1.y / v2.y
	v.z = v1.z / v2.z
	v.w = v1.w / v2.w
	return
}

func (v1 vec4D) divSc(sc float) (v vec4D) {
	v.x = v1.x / sc
	v.y = v1.y / sc
	v.z = v1.z / sc
	v.w = v1.w / sc
	return
}

func dot2D(v1, v2 vec2D) float {
	return v1.x*v2.x + v1.y*v2.y
}

func dot3D(v1, v2 vec3D) float {
	return v1.x*v2.x + v1.y*v2.y + v1.z*v2.z
}

func length2D(v vec2D) float {
	return math.Sqrt(v.x*v.x + v.y*v.y)
}

func length3D(v vec3D) float {
	return math.Sqrt(v.x*v.x + v.y*v.y + v.z*v.z)
}

func length4D(v vec4D) float {
	return math.Sqrt(v.x*v.x + v.y*v.y + v.z*v.z + v.w*v.w)
}
