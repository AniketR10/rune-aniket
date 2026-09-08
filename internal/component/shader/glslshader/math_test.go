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
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMath1Arg(t *testing.T) {
	tsuite := []struct {
		name                string
		fn                  func(in float) float
		fn2D                func(in vec2D) vec2D
		fn3D                func(in vec3D) vec3D
		fn4D                func(in vec4D) vec4D
		ins                 []float
		outs                []float
		approximateEquality bool
	}{
		{
			name:                "sin",
			fn:                  sin,
			fn2D:                sin2D,
			fn3D:                sin3D,
			fn4D:                sin4D,
			ins:                 []float{0.0, math.Pi / 2, math.Pi, 3 * math.Pi / 2, 2 * math.Pi},
			outs:                []float{0.0, 1.0, 0.0, -1.0, 0.0},
			approximateEquality: true,
		},
		{
			name:                "cos",
			fn:                  cos,
			fn2D:                cos2D,
			fn3D:                cos3D,
			fn4D:                cos4D,
			ins:                 []float{0.0, math.Pi / 2, math.Pi, 3 * math.Pi / 2, 2 * math.Pi},
			outs:                []float{1.0, 0.0, -1.0, 0.0, 1.0},
			approximateEquality: true,
		},
		{
			name:                "fract",
			fn:                  fract,
			fn2D:                fract2D,
			fn3D:                fract3D,
			fn4D:                fract4D,
			ins:                 []float{0.0, 0.5, 1.0, 1.2, 1.5, 10000.12345678},
			outs:                []float{0.0, 0.5, 0.0, 0.2, 0.5, 0.12345678},
			approximateEquality: true,
		},
		{
			name: "abs",
			fn:   abs,
			fn2D: abs2D,
			fn3D: abs3D,
			fn4D: abs4D,
			ins:  []float{0.0, 0.25, 0.5, -0.5},
			outs: []float{0.0, 0.25, 0.5, 0.5},
		},
		{
			name: "floor",
			fn:   floor,
			fn2D: floor2D,
			fn3D: floor3D,
			fn4D: floor4D,
			ins:  []float{0.0, 0.5, 1.5, -2.4},
			outs: []float{0.0, 0.0, 1.0, -3.0},
		},
	}

	for _, tcase := range tsuite {
		require.True(t, len(tcase.ins) >= 4, tcase.name+": at least have 4 values to test your func with")
		require.True(t, len(tcase.outs) >= 4, tcase.name+": at least have 4 values to test your func with")

		t.Run("1D_"+tcase.name, func(t *testing.T) {
			for i := 0; i < len(tcase.ins); i++ {
				in := tcase.ins[i]
				out := tcase.fn(in)

				ins := []float{in}
				expect := []float{tcase.outs[i]}
				result := []float{out}

				assertMulti(t, tcase.approximateEquality, ins, expect, result)
			}
		})

		t.Run("2D_"+tcase.name, func(t *testing.T) {
			in := vec2(tcase.ins[0], tcase.ins[1])
			out := tcase.fn2D(in)

			ins := []float{in.x, in.y}
			expect := []float{tcase.outs[0], tcase.outs[1]}
			result := []float{out.x, out.y}

			assertMulti(t, tcase.approximateEquality, ins, expect, result)
		})

		t.Run("3D_"+tcase.name, func(t *testing.T) {
			in := vec3(tcase.ins[0], tcase.ins[1], tcase.ins[2])
			out := tcase.fn3D(in)

			ins := []float{in.x, in.y, in.z}
			expect := []float{tcase.outs[0], tcase.outs[1], tcase.outs[2]}
			result := []float{out.x, out.y, out.z}

			assertMulti(t, tcase.approximateEquality, ins, expect, result)
		})
	}
}

func TestMax(t *testing.T) {
	tsuite := []struct {
		name string
		ins  [][]float
		outs []float
	}{
		{
			name: "max different values",
			ins:  [][]float{{1.0, 2.0}, {3.0, -1.0}},
			outs: []float{2.0, 3.0},
		},
		{
			name: "max same value",
			ins:  [][]float{{1.0, 1.0}},
			outs: []float{1.0},
		},
		{
			name: "max negative values",
			ins:  [][]float{{-1.0, -2.0}, {-10.0, 3.0}},
			outs: []float{-1.0, 3.0},
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			require.True(t, len(tcase.ins) == len(tcase.outs),
				"mismatch between inputs (%d) and outputs (%d)",
				len(tcase.ins), len(tcase.outs),
			)
			for i, in := range tcase.ins {
				res := max(in[0], in[1])
				assert.Equal(t, tcase.outs[i], res)
			}
		})
	}

	t.Run("2D", func(t *testing.T) {
		res := max2D(vec2(1.0, 4.0), vec2(2.0, 3.0))
		assert.Equal(t, 2.0, res.x)
		assert.Equal(t, 4.0, res.y)
	})

	t.Run("3D", func(t *testing.T) {
		res := max3D(vec3(1.0, 4.0, 6.0), vec3(2.0, 3.0, 5.0))
		assert.Equal(t, 2.0, res.x)
		assert.Equal(t, 4.0, res.y)
		assert.Equal(t, 6.0, res.z)
	})

	t.Run("4D", func(t *testing.T) {
		res := max4D(vec4(1.0, 4.0, 6.0, 7.0), vec4(2.0, 3.0, 5.0, 8.0))
		assert.Equal(t, 2.0, res.x)
		assert.Equal(t, 4.0, res.y)
		assert.Equal(t, 6.0, res.z)
		assert.Equal(t, 8.0, res.w)
	})
}

func TestMin(t *testing.T) {
	tsuite := []struct {
		name string
		ins  [][]float
		outs []float
	}{
		{
			name: "min different values",
			ins:  [][]float{{1.0, 2.0}, {3.0, -1.0}},
			outs: []float{1.0, -1.0},
		},
		{
			name: "min same value",
			ins:  [][]float{{1.0, 1.0}},
			outs: []float{1.0},
		},
		{
			name: "min negative values",
			ins:  [][]float{{-1.0, -2.0}, {-10.0, 3.0}},
			outs: []float{-2.0, -10.0},
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			require.True(t, len(tcase.ins) == len(tcase.outs),
				"mismatch between inputs (%d) and outputs (%d)",
				len(tcase.ins), len(tcase.outs),
			)
			for i, in := range tcase.ins {
				res := min(in[0], in[1])
				assert.Equal(t, tcase.outs[i], res)
			}
		})
	}

	t.Run("2D", func(t *testing.T) {
		res := min2D(vec2(1.0, 4.0), vec2(2.0, 3.0))
		assert.Equal(t, 1.0, res.x)
		assert.Equal(t, 3.0, res.y)
	})

	t.Run("3D", func(t *testing.T) {
		res := min3D(vec3(1.0, 4.0, 6.0), vec3(2.0, 3.0, 5.0))
		assert.Equal(t, res.x, 1.0)
		assert.Equal(t, res.y, 3.0)
		assert.Equal(t, res.z, 5.0)
	})

	t.Run("4D", func(t *testing.T) {
		res := min4D(vec4(1.0, 4.0, 6.0, 7.0), vec4(2.0, 3.0, 5.0, 8.0))
		assert.Equal(t, 1.0, res.x)
		assert.Equal(t, 3.0, res.y)
		assert.Equal(t, 5.0, res.z)
		assert.Equal(t, 7.0, res.w)
	})
}

func TestMix(t *testing.T) {
	tsuite := []struct {
		name string
		ins  [][]float
		outs []float
	}{
		{
			name: "mix positive values",
			ins:  [][]float{{0.0, 100.0, 0.25}, {100.0, 200.0, 0.75}},
			outs: []float{25.0, 175.0},
		},
		{
			name: "mix with flipped ranges and negative values",
			ins:  [][]float{{-100.0, 100.0, 0.25}, {-300.0, -200.0, 0.5}},
			outs: []float{-50.0, -250.0},
		},
		{
			name: "endpoints are equal to bounds",
			ins:  [][]float{{0.0, 100.0, 0.0}, {0.0, 100.0, 1.0}},
			outs: []float{0.0, 100.0},
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			require.True(t, len(tcase.ins) == len(tcase.outs),
				"mismatch between inputs (%d) and outputs (%d)",
				len(tcase.ins), len(tcase.outs),
			)
			for i, in := range tcase.ins {
				res := mix(in[0], in[1], in[2])
				assert.Equal(t, tcase.outs[i], res)
			}
		})
	}

	t.Run("2D", func(t *testing.T) {
		res := mix2D(vec2(0.0, -100.0), vec2(100.0, 0.0), 0.25)
		assert.Equal(t, 25.0, res.x)
		assert.Equal(t, -75.0, res.y)
	})

	t.Run("3D", func(t *testing.T) {
		res := mix3D(vec3(0.0, -100.0, 0.0), vec3(100.0, 0.0, 1000.0), 0.25)
		assert.Equal(t, 25.0, res.x)
		assert.Equal(t, -75.0, res.y)
		assert.Equal(t, 250.0, res.z)
	})

	t.Run("4D", func(t *testing.T) {
		res := mix4D(vec4(0.0, -100.0, 0.0, 0.0), vec4(100.0, 0.0, 1000.0, 0.1), 0.25)
		assert.Equal(t, 25.0, res.x)
		assert.Equal(t, -75.0, res.y)
		assert.Equal(t, 250.0, res.z)
		assert.Equal(t, 0.025, res.w)
	})
}

func TestStep(t *testing.T) {
	tsuite := []struct {
		name string
		ins  [][]float
		outs []float
	}{
		{
			name: "before step is zero after step is one",
			ins:  [][]float{{0.3, 0.2}, {0.3, 0.4}},
			outs: []float{0.0, 1.0},
		},
		{
			name: "negative values",
			ins:  [][]float{{-0.3, -0.2}, {-0.3, -0.4}},
			outs: []float{1.0, 0.0},
		},
		{
			name: "same values",
			ins:  [][]float{{-0.3, -0.3}, {0.2, 0.2}},
			outs: []float{1.0, 1.0},
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			require.True(t, len(tcase.ins) == len(tcase.outs),
				"mismatch between inputs (%d) and outputs (%d)",
				len(tcase.ins), len(tcase.outs),
			)

			for i, in := range tcase.ins {
				res := step(in[0], in[1])
				assert.Equal(t, tcase.outs[i], res)
			}
		})
	}

	t.Run("2D", func(t *testing.T) {
		res := step2D(vec2(1.0, 2.0), vec2(0.9, 2.1))
		assert.Equal(t, 0.0, res.x)
		assert.Equal(t, 1.0, res.y)
	})

	t.Run("3D", func(t *testing.T) {
		res := step3D(vec3(1.0, 2.0, -5.0), vec3(0.9, 2.1, -5.0))
		assert.Equal(t, 0.0, res.x)
		assert.Equal(t, 1.0, res.y)
		assert.Equal(t, 1.0, res.z)
	})

	t.Run("4D", func(t *testing.T) {
		res := step4D(vec4(1.0, 2.0, -5.0, 3.0), vec4(0.9, 2.1, -5.0, 2.0))
		assert.Equal(t, 0.0, res.x)
		assert.Equal(t, 1.0, res.y)
		assert.Equal(t, 1.0, res.z)
		assert.Equal(t, 0.0, res.w)
	})
}

func TestSmoothStep(t *testing.T) {
	tsuite := []struct {
		name string
		ins  [][]float
		outs []float
	}{
		{
			name: "before smoothstep first edge is one",
			ins:  [][]float{{0.4, 0.6, 0.1}},
			outs: []float{0.0},
		},
		{
			name: "after smoothstep second edge is zero",
			ins:  [][]float{{0.4, 0.6, 0.9}},
			outs: []float{1.0},
		},
		{
			name: "in the exact middle of the edges the output is 0.5",
			ins:  [][]float{{1.4, 1.6, 1.5}},
			outs: []float{0.5},
		},
		{
			name: "inbetween smoothstep edges there's interpolation between 0-1",
			ins: [][]float{
				{0.4, 0.6, 0.4}, {0.4, 0.6, 0.45}, {0.4, 0.6, 0.5},
				{0.4, 0.6, 0.55}, {0.4, 0.6, 0.6}},
			outs: []float{0.0, 0.15625, 0.5, 0.8437500000000004, 1.0},
		},
		{
			name: "first edge is greater than second edge",
			ins: [][]float{
				{0.6, 0.4, 0.4}, {0.6, 0.4, 0.45}, {0.6, 0.4, 0.5},
				{0.6, 0.4, 0.55}, {0.6, 0.4, 0.6}},
			outs: []float{1.0, 0.84375, 0.5, 0.15624999999999967, 0.0},
		},
		{
			name: "negative ranges",
			ins: [][]float{
				{-0.6, -0.4, -0.4}, {-0.6, -0.4, -0.45}, {-0.6, -0.4, -0.5},
				{-0.6, -0.4, -0.55}, {-0.6, -0.4, -0.6}},
			outs: []float{1.0, 0.84375, 0.5, 0.15624999999999967, 0.0},
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			require.True(t, len(tcase.ins) == len(tcase.outs),
				"mismatch between inputs (%d) and outputs (%d)",
				len(tcase.ins), len(tcase.outs),
			)

			for i, in := range tcase.ins {
				res := smoothstep(in[0], in[1], in[2])
				assert.Equal(t, tcase.outs[i], res)
			}
		})
	}

	t.Run("2D", func(t *testing.T) {
		res := smoothstep2D(
			vec2(1.0, 10.0), vec2(2.0, 20.0), vec2(1.2, 14.0),
		)
		assert.Equal(t, 0.10399999999999995, res.x)
		assert.Equal(t, 0.3520000000000001, res.y)
	})

	t.Run("3D", func(t *testing.T) {
		res := smoothstep3D(
			vec3(1.0, 10.0, 1000.0), vec3(2.0, 20.0, 2000.0), vec3(1.2, 14.0, 1600.0),
		)
		assert.Equal(t, 0.10399999999999995, res.x)
		assert.Equal(t, 0.3520000000000001, res.y)
		assert.Equal(t, 0.648, res.z)
	})

	t.Run("4D", func(t *testing.T) {
		res := smoothstep4D(
			vec4(1.0, 10.0, 1000.0, -5.0), vec4(2.0, 20.0, 2000.0, -10.0), vec4(1.2, 14.0, 1600.0, -7.5),
		)
		assert.Equal(t, 0.10399999999999995, res.x)
		assert.Equal(t, 0.3520000000000001, res.y)
		assert.Equal(t, 0.648, res.z)
		assert.Equal(t, 0.5, res.w)
	})
}

func TestClamp(t *testing.T) {
	tsuite := []struct {
		name string
		ins  [][]float
		outs []float
	}{
		{
			name: "value in range no clamp",
			ins:  [][]float{{4.0, 0.0, 10.0}},
			outs: []float{4.0},
		},
		{
			name: "value below range clamped",
			ins:  [][]float{{-4.0, 0.0, 10.0}},
			outs: []float{0.0},
		},
		{
			name: "value above range clamped",
			ins:  [][]float{{14.0, 0.0, 10.0}},
			outs: []float{10.0},
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			require.True(t, len(tcase.ins) == len(tcase.outs),
				"mismatch between inputs (%d) and outputs (%d)",
				len(tcase.ins), len(tcase.outs),
			)
			for i, in := range tcase.ins {
				res := clamp(in[0], in[1], in[2])
				assert.Equal(t, tcase.outs[i], res)
			}
		})
	}

	t.Run("2D", func(t *testing.T) {
		res := clamp2D(vec2(0.7, -40.0), vec2(0.0, 100.0), vec2(1.0, 200.0))
		assert.Equal(t, 0.7, res.x)
		assert.Equal(t, 100.0, res.y)
	})

	t.Run("3D", func(t *testing.T) {
		res := clamp3D(vec3(0.7, -40.0, 777.0), vec3(0.0, 100.0, 10.0), vec3(1.0, 200.0, 30.0))
		assert.Equal(t, 0.7, res.x)
		assert.Equal(t, 100.0, res.y)
		assert.Equal(t, 30.0, res.z)
	})

	t.Run("4D", func(t *testing.T) {
		res := clamp4D(vec4(0.7, -40.0, 777.0, -9.0), vec4(0.0, 100.0, 10.0, -8.0), vec4(1.0, 200.0, 30.0, -6.0))
		assert.Equal(t, 0.7, res.x)
		assert.Equal(t, 100.0, res.y)
		assert.Equal(t, 30.0, res.z)
		assert.Equal(t, -8.0, res.w)
	})
}

func almostEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) <= epsilon
}

func assertMulti(
	t *testing.T, approximateEquality bool, in []float, expect []float, actual []float,
) {
	if approximateEquality {
		for i, act := range actual {
			assert.True(t, almostEqual(expect[i], act, 0.00000001),
				"not almost equal: %f (in) %f (expected) %f (actual)",
				in[i], expect[i], act,
			)

		}
	} else {
		for i, act := range actual {
			assert.Equal(t, expect[i], act,
				"not equal: %f (in) %f (expected) %f (actual)",
				in[i], expect[i], act,
			)
		}
	}
}
