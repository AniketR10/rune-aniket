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
	"math"
	"sync/atomic"

	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/component/shader/shaderutils"
)

// GrayFadeParams configures the [GrayFade] effect.
//
// GrayFade has no color parameters: it desaturates each cell's foreground
// toward its own per-channel luminance, so every cell keeps its relative
// brightness instead of collapsing to a single shared gray. Background
// attributes are left untouched.
type GrayFadeParams struct {
	// FadeFrames decouples the visual fade length from the shader's
	// containing-component lifetime.
	//
	//   - FadeFrames > 0: progress ramps linearly from 0 to 1 over
	//     FadeFrames frames and then HOLDS at 1.0 until frame >= total.
	//     Use this when the shader has to outlive its visual animation —
	//     e.g. as the workspace-loading shader, where the host component
	//     is configured with an oversized total to cover the async load,
	//     but the fade itself should finish in a fixed wall-clock budget
	//     and stay there.
	//
	//   - FadeFrames == 0 (default): standard [Shader] convention.
	//     Progress ramps linearly with frame/(total-1) and reaches 1.0
	//     at frame == total-1. Use this for standalone runs where the
	//     caller-supplied duration is the only knob (e.g. `:shaderrun
	//     grayFade <duration>`).
	FadeFrames int
}

// DefaultGrayFadeParams returns a sane set of [GrayFadeParams]. The
// default leaves [GrayFadeParams.FadeFrames] zero so GrayFade behaves
// like a normal duration-driven shader; callers that need the
// fade-then-hold behavior set it explicitly.
func DefaultGrayFadeParams() GrayFadeParams { return GrayFadeParams{} }

// GrayFade is a Shader that progressively removes color from every cell's
// foreground by desaturating it toward its own luminance gray. It is
// intended to be used as the "loading" stage of a transition; the next
// shader can query [GrayFadeShader.Progress] to find out how far the
// desaturation has gone and apply the same amount to its own snapshot,
// so the two effects compose without a visible seam.
func GrayFade(params GrayFadeParams, defaultAttr term.Attributes) *GrayFadeShader {
	return &GrayFadeShader{GrayFadeParams: params, defaultAttr: defaultAttr}
}

// GrayFadeShader is the concrete [Shader] type returned by [GrayFade].
// It is exported so callers can recover its [GrayFadeShader.Progress]
// from a [Shader] interface value via a type assertion.
type GrayFadeShader struct {
	GrayFadeParams
	defaultAttr term.Attributes

	// progressBits stores the latest [0,1] progress observed during
	// Shade encoded as math.Float64bits so it can be read atomically by
	// callers (e.g. shaderRunner) without locking the draw thread.
	progressBits atomic.Uint64
}

// Shade satisfies [Shader]. See [GrayFadeParams.FadeFrames] for the two
// progress regimes. Frame 0 always leaves cells untouched so the first
// frame of the transition is the live content.
func (s *GrayFadeShader) Shade(frame, total int, cells [][]term.Cell) {
	if frame < 0 {
		return
	}
	rampFrames := s.FadeFrames
	if rampFrames <= 0 {
		// Standard shader semantics: ramp ends at frame == total-1.
		if total <= 0 {
			return
		}
		rampFrames = total
	}
	progress := 1.0
	switch {
	case rampFrames <= 1:
		progress = 1
	case frame < rampFrames-1:
		progress = float64(frame) / float64(rampFrames-1)
	}
	progress = math.Min(1, math.Max(0, progress))
	s.setProgress(progress)
	if progress <= 0 {
		return
	}
	for y, row := range cells {
		for x := range row {
			cells[y][x].Fg = shaderutils.DesaturateColor(
				row[x].Fg, progress, s.defaultAttr.Fg,
			)
		}
	}
}

// Progress returns the latest fade progress observed by Shade, in the
// range [0, 1]. It is safe to call from another goroutine; the value is
// always consistent with the most recent draw.
func (s *GrayFadeShader) Progress() float64 {
	return math.Float64frombits(s.progressBits.Load())
}

func (s *GrayFadeShader) setProgress(p float64) {
	s.progressBits.Store(math.Float64bits(p))
}
