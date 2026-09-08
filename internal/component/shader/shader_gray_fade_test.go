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

package shader_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/component/shader"
)

func TestGrayFade_FirstFrameLeavesCellsIntact(t *testing.T) {
	sh := shader.GrayFade(shader.DefaultGrayFadeParams(), term.Attributes{})
	in := makeCharCells(4, 2)
	in[0][0].Fg = term.NewRGBColor(255, 0, 0)
	want := cloneCells(in)

	sh.Shade(0, 10, in)

	assert.Equal(t, want, in, "frame 0 must not alter any cell")
	assert.Equal(t, 0.0, sh.Progress())
}

func TestGrayFade_HoldsAtFullDesaturationAfterFadeFrames(t *testing.T) {
	// The loading shader must NOT regain color if addWorkspace takes
	// longer than the configured fade window. After FadeFrames it must
	// keep Progress() pinned at 1.0 so the open shader can chain from
	// the same fully-desaturated state.
	params := shader.DefaultGrayFadeParams()
	params.FadeFrames = 5
	sh := shader.GrayFade(params, term.Attributes{})

	// total is intentionally much larger than FadeFrames; the shader
	// runner uses an oversized total so the Component never expires.
	const total = 10_000

	// At the end of the fade window: full desaturation.
	in := makeCharCells(2, 1)
	in[0][0].Fg = term.NewRGBColor(255, 0, 0)
	sh.Shade(params.FadeFrames-1, total, in)
	assert.Equal(t, 1.0, sh.Progress())
	r1, g1, b1 := in[0][0].Fg.RGB()
	assert.Equal(t, r1, g1, "Fg must be fully desaturated at end of fade")
	assert.Equal(t, r1, b1)

	// Many frames later: still fully desaturated, no recolor.
	in = makeCharCells(2, 1)
	in[0][0].Fg = term.NewRGBColor(255, 0, 0)
	sh.Shade(500, total, in)
	assert.Equal(t, 1.0, sh.Progress(),
		"Progress must stay at 1.0 past FadeFrames")
	r2, g2, b2 := in[0][0].Fg.RGB()
	assert.Equal(t, r1, r2, "color must not drift after the fade completes")
	assert.Equal(t, r2, g2)
	assert.Equal(t, r2, b2)
}

func TestGrayFade_LastFrameFullyDesaturatesFg(t *testing.T) {
	// The loading shader must finish at Progress()==1 so the open shader
	// can pick up exactly where it left off. Background is left alone.
	params := shader.DefaultGrayFadeParams()
	params.FadeFrames = 10
	sh := shader.GrayFade(params, term.Attributes{})
	in := makeCharCells(2, 1)
	in[0][0].Fg = term.NewRGBColor(255, 0, 0)
	in[0][0].Bg = term.NewRGBColor(10, 20, 30)

	sh.Shade(params.FadeFrames-1, 10_000, in)

	assert.Equal(t, 1.0, sh.Progress())
	r, g, b := in[0][0].Fg.RGB()
	assert.Equal(t, r, g, "fully-desaturated Fg should have r==g==b")
	assert.Equal(t, r, b, "fully-desaturated Fg should have r==g==b")
	assert.Equal(t, term.NewRGBColor(10, 20, 30), in[0][0].Bg,
		"GrayFade must not alter the background")
}

func TestGrayFade_DesaturationPreservesPerCellBrightness(t *testing.T) {
	// The whole point of using per-cell desaturation instead of blending
	// toward a single shared gray is that distinct source colors stay
	// distinct (and visible) after the fade.
	params := shader.DefaultGrayFadeParams()
	params.FadeFrames = 10
	sh := shader.GrayFade(params, term.Attributes{})
	in := makeCharCells(2, 1)
	in[0][0].Fg = term.NewRGBColor(255, 0, 0)
	in[0][1].Fg = term.NewRGBColor(255, 255, 255)

	sh.Shade(params.FadeFrames-1, 10_000, in)

	left, _, _ := in[0][0].Fg.RGB()
	right, _, _ := in[0][1].Fg.RGB()
	assert.NotEqual(t, left, right,
		"different source colors must not collapse to the same gray")
}

func TestGrayFade_DefaultParamsRampOverTotal(t *testing.T) {
	// When FadeFrames is left at its zero value, GrayFade falls back to
	// the standard Shader convention: progress ramps linearly with
	// frame/(total-1) and reaches 1 at frame==total-1. This lets the
	// shader be used standalone (e.g. via :shaderrun grayFade) where the
	// caller-supplied duration is the only knob.
	sh := shader.GrayFade(shader.GrayFadeParams{}, term.Attributes{})

	const total = 10
	in := makeCharCells(2, 1)
	in[0][0].Fg = term.NewRGBColor(255, 0, 0)
	sh.Shade(total-1, total, in)
	assert.Equal(t, 1.0, sh.Progress(),
		"default params should reach full progress at frame==total-1")
	r, g, b := in[0][0].Fg.RGB()
	assert.Equal(t, r, g)
	assert.Equal(t, r, b)

	// Mid-way through total, progress should be partial.
	sh = shader.GrayFade(shader.GrayFadeParams{}, term.Attributes{})
	mid := makeCharCells(2, 1)
	mid[0][0].Fg = term.NewRGBColor(255, 0, 0)
	sh.Shade(total/2, total, mid)
	assert.InDelta(t, float64(total/2)/float64(total-1), sh.Progress(), 1e-9)
}

func TestBurn_FrameZeroPaintsInitialSnapshot(t *testing.T) {
	// Frame 0 of the open shader must already be the captured snapshot,
	// otherwise the live workspace flashes for one frame at the handover
	// before the burn wave begins. This is what makes the loading→open
	// transition feel continuous.
	params := shader.DefaultBurnParams()
	params.FixedOrigin = true
	params.OriginX = 0.5
	params.OriginY = 0.5
	sh := shader.Burn(params, term.Attributes{})

	type cellSetter interface {
		SetInitialCells([][]term.Cell)
	}
	snapshot := makeCharCells(4, 2)
	for y := range snapshot {
		for x := range snapshot[y] {
			snapshot[y][x].Ch = 'O'
		}
	}
	sh.(cellSetter).SetInitialCells(snapshot)

	live := makeCharCells(4, 2)
	for y := range live {
		for x := range live[y] {
			live[y][x].Ch = 'N'
		}
	}
	sh.Shade(0, 30, live)

	for y := range live {
		for x := range live[y] {
			assert.Equalf(t, 'O', live[y][x].Ch,
				"frame 0 should show the snapshot at (%d,%d)", x, y)
		}
	}
}

func TestBurn_SetInitialDesaturationAppliesToSnapshot(t *testing.T) {
	// SetInitialDesaturation is what carries the loading shader's
	// progress into the open shader: at frame 0 the snapshot rendered by
	// Burn must match the gray-fade end state.
	params := shader.DefaultBurnParams()
	params.FixedOrigin = true
	params.OriginX = 0.5
	params.OriginY = 0.5
	sh := shader.Burn(params, term.Attributes{})

	type setter interface {
		SetInitialCells([][]term.Cell)
		SetInitialDesaturation(amount float64)
	}
	snapshot := makeCharCells(2, 1)
	snapshot[0][0].Fg = term.NewRGBColor(255, 0, 0)
	snapshot[0][1].Fg = term.NewRGBColor(0, 255, 0)
	sh.(setter).SetInitialCells(snapshot)
	sh.(setter).SetInitialDesaturation(1)

	live := makeCharCells(2, 1)
	sh.Shade(0, 30, live)

	lr, lg, lb := live[0][0].Fg.RGB()
	rr, rg, rb := live[0][1].Fg.RGB()
	assert.Equal(t, lr, lg)
	assert.Equal(t, lr, lb)
	assert.Equal(t, rr, rg)
	assert.Equal(t, rr, rb)
	assert.NotEqual(t, lr, rr,
		"distinct source colors should remain distinct grays")
}
