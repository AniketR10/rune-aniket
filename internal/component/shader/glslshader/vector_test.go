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

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVector(t *testing.T) {
	t.Run("order of components in vector construction", func(t *testing.T) {
		v2 := vec2(1.1, 2.2)
		assert.Equal(t, 1.1, v2.x)
		assert.Equal(t, 2.2, v2.y)

		v3 := vec3(1.1, 2.2, 3.3)
		assert.Equal(t, 1.1, v3.x)
		assert.Equal(t, 2.2, v3.y)
		assert.Equal(t, 3.3, v3.z)

		v4 := vec4(1.1, 2.2, 3.3, 4.4)
		assert.Equal(t, 1.1, v4.x)
		assert.Equal(t, 2.2, v4.y)
		assert.Equal(t, 3.3, v4.z)
		assert.Equal(t, 4.4, v4.w)
	})

	t.Run("construct vector with one arg repeats it to all components", func(t *testing.T) {
		v2 := vec2FromScalar(2.2)
		assert.Equal(t, 2.2, v2.x)
		assert.Equal(t, 2.2, v2.y)

		v3 := vec3FromScalar(3.3)
		assert.Equal(t, 3.3, v3.x)
		assert.Equal(t, 3.3, v3.y)
		assert.Equal(t, 3.3, v3.z)

		v4 := vec4(4.4)
		assert.Equal(t, 4.4, v4.x)
		assert.Equal(t, 4.4, v4.y)
		assert.Equal(t, 4.4, v4.z)
		assert.Equal(t, 4.4, v4.w)
	})
}

func TestVectorAddSubtract(t *testing.T) {
	tsuite2D := []struct {
		name      string
		operand1  vec2D
		operand2  vec2D
		expectAdd vec2D
		expectSub vec2D
	}{
		{
			name:      "2D adding and subtracting component-wise",
			operand1:  vec2(1.1, 2.2),
			operand2:  vec2(3.3, 4.4),
			expectAdd: vec2(4.4, 6.6000000000000005),
			expectSub: vec2(-2.1999999999999997, -2.2),
		},
		{
			name:      "2D float overflowing results in infinity",
			operand1:  vec2(math.MaxFloat64, -math.MaxFloat64),
			operand2:  vec2(math.MaxFloat64, math.MaxFloat64),
			expectAdd: vec2(math.Inf(1), 0.0),
			expectSub: vec2(0.0, math.Inf(-1)),
		},
	}

	for _, tcase := range tsuite2D {
		t.Run(tcase.name, func(t *testing.T) {
			resAdd := tcase.operand1.add(tcase.operand2)
			assert.Equal(t, tcase.expectAdd, resAdd)

			resSub := tcase.operand1.sub(tcase.operand2)
			assert.Equal(t, tcase.expectSub, resSub)
		})
	}

	tsuite3D := []struct {
		name      string
		operand1  vec3D
		operand2  vec3D
		expectAdd vec3D
		expectSub vec3D
	}{
		{
			name:      "3D adding and subtracting component-wise",
			operand1:  vec3(1.7, 8.0, 2.8),
			operand2:  vec3(4.0, 1.1, 6.0),
			expectAdd: vec3(5.7, 9.1, 8.8),
			expectSub: vec3(-2.3, 6.9, -3.2),
		},
		{
			name:      "3D float overflowing results in infinity",
			operand1:  vec3(math.MaxFloat64, -math.MaxFloat64, math.MaxFloat64),
			operand2:  vec3(math.MaxFloat64, math.MaxFloat64, math.MaxFloat64),
			expectAdd: vec3(math.Inf(1), 0.0, math.Inf(1)),
			expectSub: vec3(0.0, math.Inf(-1), 0.0),
		},
	}

	for _, tcase := range tsuite3D {
		t.Run(tcase.name, func(t *testing.T) {
			resAdd := tcase.operand1.add(tcase.operand2)
			assert.Equal(t, tcase.expectAdd, resAdd)

			resSub := tcase.operand1.sub(tcase.operand2)
			assert.Equal(t, tcase.expectSub, resSub)
		})
	}

	tsuite4D := []struct {
		name      string
		operand1  vec4D
		operand2  vec4D
		expectAdd vec4D
		expectSub vec4D
	}{
		{
			name:      "4D adding and subtracting component-wise",
			operand1:  vec4(-6.9, 8.3, -22.0, 5.4),
			operand2:  vec4(12.9, -81.1, 63.0, 3.2),
			expectAdd: vec4(6.0, -72.8, 41, 8.600000000000001),
			expectSub: vec4(-19.8, 89.39999999999999, -85, 2.2),
		},
		{
			name:      "4D float overflowing results in infinity",
			operand1:  vec4(math.MaxFloat64, -math.MaxFloat64, math.MaxFloat64, -math.MaxFloat64),
			operand2:  vec4(math.MaxFloat64, math.MaxFloat64, math.MaxFloat64, math.MaxFloat64),
			expectAdd: vec4(math.Inf(1), 0.0, math.Inf(1), 0.0),
			expectSub: vec4(0.0, math.Inf(-1), 0.0, math.Inf(-1)),
		},
	}

	for _, tcase := range tsuite4D {
		t.Run(tcase.name, func(t *testing.T) {
			resAdd := tcase.operand1.add(tcase.operand2)
			assert.Equal(t, tcase.expectAdd, resAdd)

			resSub := tcase.operand1.sub(tcase.operand2)
			assert.Equal(t, tcase.expectSub, resSub)
		})
	}

	t.Run("2D add scalar", func(t *testing.T) {
		assert.Equal(t, vec2(2.5, 3.5), vec2(1.0, 2.0).addSc(1.5))
	})

	t.Run("3D add scalar", func(t *testing.T) {
		assert.Equal(t, vec3(2.5, 3.5, 4.5), vec3(1.0, 2.0, 3.0).addSc(1.5))
	})

	t.Run("4D add scalar", func(t *testing.T) {
		assert.Equal(t, vec4(2.5, 3.5, 4.5, 5.5), vec4(1.0, 2.0, 3.0, 4.0).addSc(1.5))
	})

	t.Run("2D subtract scalar", func(t *testing.T) {
		assert.Equal(t, vec2(-0.5, 0.5), vec2(1.0, 2.0).subSc(1.5))
	})

	t.Run("3D subtract scalar", func(t *testing.T) {
		assert.Equal(t, vec3(-0.5, 0.5, 1.5), vec3(1.0, 2.0, 3.0).subSc(1.5))
	})

	t.Run("4D subtract scalar", func(t *testing.T) {
		assert.Equal(t, vec4(-0.5, 0.5, 1.5, 2.5), vec4(1.0, 2.0, 3.0, 4.0).subSc(1.5))
	})
}

func TestVectorMult(t *testing.T) {
	tsuiteMult2D := []struct {
		name     string
		operand1 vec2D
		operand2 vec2D
		expect   vec2D
	}{
		{
			name:     "2D component-wise multiplication",
			operand1: vec2(3.5, 2.2),
			operand2: vec2(2.0, -2.0),
			expect:   vec2(7.0, -4.4),
		},
		{
			name:     "2D float overflow multiplication yields infinity",
			operand1: vec2(math.MaxFloat64, math.MaxFloat64),
			operand2: vec2(2.0, -2.0),
			expect:   vec2(math.Inf(1), math.Inf(-1)),
		},
	}

	for _, tcase := range tsuiteMult2D {
		t.Run(tcase.name, func(t *testing.T) {
			res := tcase.operand1.mult(tcase.operand2)
			assert.Equal(t, tcase.expect, res)
		})
	}

	tsuiteMult3D := []struct {
		name     string
		operand1 vec3D
		operand2 vec3D
		expect   vec3D
	}{
		{
			name:     "3D component-wise multiplication",
			operand1: vec3(3.5, 2.2, 5.2),
			operand2: vec3(2.0, -2.0, 3.0),
			expect:   vec3(7.0, -4.4, 15.600000000000001),
		},
		{
			name:     "3D float overflow multiplication yields infinity",
			operand1: vec3(math.MaxFloat64, math.MaxFloat64, math.MaxFloat64),
			operand2: vec3(2.0, -2.0, 2.0),
			expect:   vec3(math.Inf(1), math.Inf(-1), math.Inf(1)),
		},
	}

	for _, tcase := range tsuiteMult3D {
		t.Run(tcase.name, func(t *testing.T) {
			res := tcase.operand1.mult(tcase.operand2)
			assert.Equal(t, tcase.expect, res)
		})
	}

	tsuiteMult4D := []struct {
		name     string
		operand1 vec4D
		operand2 vec4D
		expect   vec4D
	}{
		{
			name:     "4D component-wise multiplication",
			operand1: vec4(3.5, 2.2, 5.2, 6.3),
			operand2: vec4(2.0, -2.0, 3.0, -3.0),
			expect:   vec4(7.0, -4.4, 15.600000000000001, -18.9),
		},
		{
			name:     "4D float overflow multiplication yields infinity",
			operand1: vec4(math.MaxFloat64, math.MaxFloat64, math.MaxFloat64, 2.0),
			operand2: vec4(2.0, -2.0, 2.0, math.MaxFloat64),
			expect:   vec4(math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(1)),
		},
	}

	for _, tcase := range tsuiteMult4D {
		t.Run(tcase.name, func(t *testing.T) {
			res := tcase.operand1.mult(tcase.operand2)
			assert.Equal(t, tcase.expect, res)
		})
	}

	t.Run("2D scalar multiplication", func(t *testing.T) {
		assert.Equal(t, vec2(7.0, 100.0), vec2(0.7, 10.0).multSc(10.0))
	})

	t.Run("3D scalar multiplication", func(t *testing.T) {
		assert.Equal(t, vec3(7.0, 100.0, -2), vec3(0.7, 10.0, -0.2).multSc(10.0))
	})

	t.Run("4D scalar multiplication", func(t *testing.T) {
		assert.Equal(t, vec4(7.0, 100.0, -2.0, 0.0), vec4(0.7, 10.0, -0.2, 0.0).multSc(10.0))
	})
}

func TestVectorDiv(t *testing.T) {
	tsuiteDiv2D := []struct {
		name     string
		operand1 vec2D
		operand2 vec2D
		expect   vec2D
	}{
		{
			name:     "2D component-wise division",
			operand1: vec2(10.5, 18.9),
			operand2: vec2(2.0, -3.0),
			expect:   vec2(5.25, -6.3),
		},
		{
			name:     "2D division by zero yields infinity or NaN",
			operand1: vec2(-1.0, 2.2),
			operand2: vec2(0.0, 0.0),
			expect:   vec2(math.Inf(-1), math.Inf(1)),
		},
	}

	for _, tcase := range tsuiteDiv2D {
		t.Run(tcase.name, func(t *testing.T) {
			res := tcase.operand1.div(tcase.operand2)
			assert.Equal(t, tcase.expect, res)
		})
	}

	tsuiteDiv3D := []struct {
		name     string
		operand1 vec3D
		operand2 vec3D
		expect   vec3D
	}{
		{
			name:     "3D component-wise division",
			operand1: vec3(10.5, 18.9, 78),
			operand2: vec3(2.0, -3.0, 1.2),
			expect:   vec3(5.25, -6.3, 65.0),
		},
		{
			name:     "3D division by zero yields infinity",
			operand1: vec3(30.0, -2.2, 40.0),
			operand2: vec3(0.0, 0.0, 0.0),
			expect:   vec3(math.Inf(1), math.Inf(-1), math.Inf(1)),
		},
	}

	for _, tcase := range tsuiteDiv3D {
		t.Run(tcase.name, func(t *testing.T) {
			res := tcase.operand1.div(tcase.operand2)
			assert.Equal(t, tcase.expect, res)
		})
	}

	tsuiteDiv4D := []struct {
		name     string
		operand1 vec4D
		operand2 vec4D
		expect   vec4D
	}{
		{
			name:     "4D component-wise division",
			operand1: vec4(10.5, 18.9, 78, 1.0),
			operand2: vec4(2.0, -3.0, 1.2, 0.5),
			expect:   vec4(5.25, -6.3, 65.0, 2.0),
		},
		{
			name:     "4D division by zero yields infinity",
			operand1: vec4(30.0, -2.2, 40.0, -22.0),
			operand2: vec4(0.0, 0.0, 0.0, 0.0),
			expect:   vec4(math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)),
		},
	}

	for _, tcase := range tsuiteDiv4D {
		t.Run(tcase.name, func(t *testing.T) {
			res := tcase.operand1.div(tcase.operand2)
			assert.Equal(t, tcase.expect, res)
		})
	}

	t.Run("2D scalar division", func(t *testing.T) {
		assert.Equal(t, vec2(4.0, -2.0), vec2(2.0, -1.0).divSc(0.5))
	})

	t.Run("3D scalar division", func(t *testing.T) {
		assert.Equal(t, vec3(4.0, -2.0, -45.2), vec3(2.0, -1.0, -22.6).divSc(0.5))
	})

	t.Run("4D scalar division", func(t *testing.T) {
		assert.Equal(t, vec4(4.0, -2.0, -45.2, 0.004), vec4(2.0, -1.0, -22.6, 0.002).divSc(0.5))
	})

	t.Run("zero divided by zero is NaN", func(t *testing.T) {
		v := vec2(0.0, 0.0).divSc(0.0)

		// cannot do v.x == math.NaN(), that's why it gets tested differenty
		assert.True(t, math.IsNaN(v.x))
		assert.True(t, math.IsNaN(v.y))
	})
}

func TestDot(t *testing.T) {
	t.Run("2D dot product", func(t *testing.T) {
		res := dot2D(vec2(0.0, 1.0), vec2(1.0, 0.0))
		assert.Equal(t, 0.0, res)

		res = dot2D(vec2(1.0, 0.0), vec2(0.0, 1.0))
		assert.Equal(t, 0.0, res)

		res = dot2D(vec2(1.0, 1.0), vec2(1.0, 1.0))
		assert.Equal(t, 2.0, res)

		res = dot2D(vec2(-1.0, -1.0), vec2(1.0, 1.0))
		assert.Equal(t, -2.0, res)
	})

	t.Run("3D dot product", func(t *testing.T) {
		res := dot3D(vec3(1.0, 1.0, 1.0), vec3(1.0, 1.0, 1.0))
		assert.Equal(t, 3.0, res)

		res = dot3D(vec3(-1.0, -1.0, -1.0), vec3(1.0, 1.0, 1.0))
		assert.Equal(t, -3.0, res)

		res = dot3D(vec3(1.0, 0.0, 0.0), vec3(0.0, 1.0, 0.0))
		assert.Equal(t, 0.0, res)
	})
}

func TestLength(t *testing.T) {
	t.Run("2D length", func(t *testing.T) {
		got := length2D(vec2(0.0, 0.0))
		assert.Equal(t, 0.0, got)

		got = length2D(vec2(1.0, 0.0))
		assert.Equal(t, 1.0, got)

		got = length2D(vec2(0.0, 1.0))
		assert.Equal(t, 1.0, got)

		got = length2D(vec2(3.0, 4.0))
		assert.Equal(t, 5.0, got)

		got = length2D(vec2(-145.43, 41.04))
		assert.InDelta(t, 151.10978293942455, got, 1e-6)

		got = length2D(vec2(1e-3, 3e3))
		assert.InDelta(t, 3000.000000000167, got, 1e-6)
	})

	t.Run("3D length", func(t *testing.T) {
		got := length3D(vec3(0.0, 0.0, 0.0))
		assert.Equal(t, 0.0, got)

		got = length3D(vec3(1.0, 0.0, 0.0))
		assert.Equal(t, 1.0, got)

		got = length3D(vec3(0.0, 1.0, 0.0))
		assert.Equal(t, 1.0, got)

		got = length3D(vec3(0.0, 0.0, 1.0))
		assert.Equal(t, 1.0, got)

		got = length3D(vec3(1.0, 2.0, 2.0))
		assert.Equal(t, 3.0, got)

		got = length3D(vec3(44.1, 68.1, 14.045))
		assert.InDelta(t, 82.33882453010851, got, 1e-6)

		got = length3D(vec3(4e9, 6e-7, 0.4))
		assert.InDelta(t, 4e9, got, 1e-6)
	})

	t.Run("4D length", func(t *testing.T) {
		got := length4D(vec4(0.0, 0.0, 0.0, 0.0))
		assert.Equal(t, 0.0, got)

		got = length4D(vec4(1.0, 0.0, 0.0, 0.0))
		assert.Equal(t, 1.0, got)

		got = length4D(vec4(0.0, 1.0, 0.0, 0.0))
		assert.Equal(t, 1.0, got)

		got = length4D(vec4(0.0, 0.0, 1.0, 0.0))
		assert.Equal(t, 1.0, got)

		got = length4D(vec4(0.0, 0.0, 0.0, 1.0))
		assert.Equal(t, 1.0, got)

		got = length4D(vec4(1.0, 2.0, 3.0, 4.0))
		assert.InDelta(t, 5.477225575051661, got, 1e-6)

		got = length4D(vec4(44.1, 68.1, 14.045, 2.93472))
		assert.InDelta(t, 82.39110756919341, got, 1e-6)

		got = length4D(vec4(3e6, 3e-5, 333.0, 0.1))
		assert.InDelta(t, 3.0000000184815014e+06, got, 1e-6)
	})
}
