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
	"math"
)

type float = float64

var mathE = 1.0e-10

func sin(x float) float {
	return math.Sin(x)
}

func sin2D(s vec2D) vec2D {
	return vec2(sin(s.x), sin(s.y))
}

func sin3D(s vec3D) vec3D {
	return vec3(sin(s.x), sin(s.y), sin(s.z))
}

func sin4D(s vec4D) vec4D {
	return vec4(sin(s.x), sin(s.y), sin(s.z), sin(s.w))
}

func cos(x float) float {
	return math.Cos(x)
}

func cos2D(s vec2D) vec2D {
	return vec2(cos(s.x), cos(s.y))
}

func cos3D(s vec3D) vec3D {
	return vec3(cos(s.x), cos(s.y), cos(s.z))
}

func cos4D(s vec4D) vec4D {
	return vec4(cos(s.x), cos(s.y), cos(s.z), cos(s.w))
}

func fract(x float) float {
	return x - math.Floor(x)
}
func fract2D(in vec2D) (v vec2D) {
	v.x = fract(in.x)
	v.y = fract(in.y)
	return
}

func fract3D(in vec3D) (v vec3D) {
	v.x = fract(in.x)
	v.y = fract(in.y)
	v.z = fract(in.z)
	return
}

func fract4D(in vec4D) (v vec4D) {
	v.x = fract(in.x)
	v.y = fract(in.y)
	v.z = fract(in.z)
	v.w = fract(in.w)
	return
}

func abs(in float) float {
	return math.Abs(in)
}

func abs2D(in vec2D) (v vec2D) {
	v.x = abs(in.x)
	v.y = abs(in.y)
	return
}

func abs3D(in vec3D) (v vec3D) {
	v.x = abs(in.x)
	v.y = abs(in.y)
	v.z = abs(in.z)
	return
}

func abs4D(in vec4D) (v vec4D) {
	v.x = abs(in.x)
	v.y = abs(in.y)
	v.z = abs(in.z)
	v.w = abs(in.w)
	return
}

func floor(in float) float {
	return math.Floor(in)
}

func floor2D(in vec2D) (v vec2D) {
	v.x = floor(in.x)
	v.y = floor(in.y)
	return
}

func floor3D(in vec3D) (v vec3D) {
	v.x = floor(in.x)
	v.y = floor(in.y)
	v.z = floor(in.z)
	return
}

func floor4D(in vec4D) (v vec4D) {
	v.x = floor(in.x)
	v.y = floor(in.y)
	v.z = floor(in.z)
	v.w = floor(in.w)
	return
}

func clamp(s, lower, upper float) float {
	if s < lower {
		return lower
	} else if s > upper {
		return upper
	} else {
		return s
	}
}

func clampInt(s, lower, upper int) int {
	if s < lower {
		return lower
	} else if s > upper {
		return upper
	} else {
		return s
	}
}

func clamp2D(s, lower, upper vec2D) (out vec2D) {
	out.x = clamp(s.x, lower.x, upper.x)
	out.y = clamp(s.y, lower.y, upper.y)
	return
}

func clamp3D(s, lower, upper vec3D) (out vec3D) {
	out.x = clamp(s.x, lower.x, upper.x)
	out.y = clamp(s.y, lower.y, upper.y)
	out.z = clamp(s.z, lower.z, upper.z)
	return
}

func clamp4D(s, lower, upper vec4D) (out vec4D) {
	out.x = clamp(s.x, lower.x, upper.x)
	out.y = clamp(s.y, lower.y, upper.y)
	out.z = clamp(s.z, lower.z, upper.z)
	out.w = clamp(s.w, lower.w, upper.w)
	return
}

func max(a, b float) float {
	return math.Max(a, b)
}

func max2D(a, b vec2D) (v vec2D) {
	v.x = max(a.x, b.x)
	v.y = max(a.y, b.y)
	return
}

func max3D(a, b vec3D) (v vec3D) {
	v.x = max(a.x, b.x)
	v.y = max(a.y, b.y)
	v.z = max(a.z, b.z)
	return
}

func max4D(a, b vec4D) (v vec4D) {
	v.x = max(a.x, b.x)
	v.y = max(a.y, b.y)
	v.z = max(a.z, b.z)
	v.w = max(a.w, b.w)
	return
}

func min(a, b float) float {
	return math.Min(a, b)
}

func min2D(a, b vec2D) (v vec2D) {
	v.x = min(a.x, b.x)
	v.y = min(a.y, b.y)
	return
}

func min3D(a, b vec3D) (v vec3D) {
	v.x = min(a.x, b.x)
	v.y = min(a.y, b.y)
	v.z = min(a.z, b.z)
	return
}

func min4D(a, b vec4D) (v vec4D) {
	v.x = min(a.x, b.x)
	v.y = min(a.y, b.y)
	v.z = min(a.z, b.z)
	v.w = min(a.w, b.w)
	return
}

func mix(a float, b float, t float) float {
	return a*(1-t) + b*t
}

func mix2D(a vec2D, b vec2D, t float) (v vec2D) {
	v.x = mix(a.x, b.x, t)
	v.y = mix(a.y, b.y, t)
	return
}

func mix3D(a vec3D, b vec3D, t float) (v vec3D) {
	v.x = mix(a.x, b.x, t)
	v.y = mix(a.y, b.y, t)
	v.z = mix(a.z, b.z, t)
	return
}

func mix4D(a vec4D, b vec4D, t float) (v vec4D) {
	v.x = mix(a.x, b.x, t)
	v.y = mix(a.y, b.y, t)
	v.z = mix(a.z, b.z, t)
	v.w = mix(a.w, b.w, t)
	return
}

func step(edge float, in float) float {
	if in < edge {
		return 0.0
	}
	return 1.0
}

func step2D(edge vec2D, in vec2D) (v vec2D) {
	v.x = step(edge.x, in.x)
	v.y = step(edge.y, in.y)
	return
}

func step3D(edge vec3D, in vec3D) (v vec3D) {
	v.x = step(edge.x, in.x)
	v.y = step(edge.y, in.y)
	v.z = step(edge.z, in.z)
	return
}

func step4D(edge vec4D, in vec4D) (v vec4D) {
	v.x = step(edge.x, in.x)
	v.y = step(edge.y, in.y)
	v.z = step(edge.z, in.z)
	v.w = step(edge.w, in.w)
	return
}

func smoothstep(edge0, edge1, in float) float {
	t := clamp((in-edge0)/(edge1-edge0), 0.0, 1.0)
	return t * t * (3.0 - 2.0*t)
}

func smoothstep2D(edge0, edge1 vec2D, in vec2D) (v vec2D) {
	v.x = smoothstep(edge0.x, edge1.x, in.x)
	v.y = smoothstep(edge0.y, edge1.y, in.y)
	return
}

func smoothstep3D(edge0, edge1 vec3D, in vec3D) (v vec3D) {
	v.x = smoothstep(edge0.x, edge1.x, in.x)
	v.y = smoothstep(edge0.y, edge1.y, in.y)
	v.z = smoothstep(edge0.z, edge1.z, in.z)
	return
}

func smoothstep4D(edge0, edge1 vec4D, in vec4D) (v vec4D) {
	v.x = smoothstep(edge0.x, edge1.x, in.x)
	v.y = smoothstep(edge0.y, edge1.y, in.y)
	v.z = smoothstep(edge0.z, edge1.z, in.z)
	v.w = smoothstep(edge0.w, edge1.w, in.w)
	return
}
