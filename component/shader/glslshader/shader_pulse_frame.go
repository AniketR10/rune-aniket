// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package glslshader

import (
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/component/shader"
	"unstable.build/go-tui/component/shader/shaderutils"
)

// PulseFrame produces a temporal pulsing effect on every cell whose character
// belongs to the configured [component.FrameCharSet]: the whole frame breathes
// in unison towards [PulseFrameParams.Color] and back to its original
// foreground.
//
// Unlike [ShineFrame] there is no spatial direction; at any given frame all
// matching cells share the same blend intensity. Use this when you want to
// emphasize the entire frame rather than draw a sweeping highlight across it.
func PulseFrame(params PulseFrameParams, defaultAttr term.Attributes) shader.Shader {
	return &pulseFrame{PulseFrameParams: params, defaultAttr: defaultAttr}
}

// PulseFrameParams allows you to customize the [PulseFrame] effect.
type PulseFrameParams struct {
	// FrameCharSet is the set of frame characters the pulse effect is
	// applied to. Cells whose character is not part of this set are left
	// untouched.
	FrameCharSet component.FrameCharSet
	// Color is the color blended into the foreground of matching cells at
	// the peak of each pulse.
	Color term.Color
	// MinIntensity is the residual blend factor at the trough of the pulse,
	// in the [0, 1] range. 0 returns the frame to its original color; values
	// closer to 1 keep more of [Color] visible between peaks.
	MinIntensity float
	// Cycles is the number of full pulse cycles played across the animation.
	// Values less than 1 are treated as 1.
	Cycles int
}

// DefaultPulseFrameParams returns a sane set of [PulseFrameParams] using the
// given [component.FrameCharSet] as the set of cells the pulse applies to.
func DefaultPulseFrameParams(fc component.FrameCharSet) PulseFrameParams {
	return PulseFrameParams{
		FrameCharSet: fc,
		Color:        term.NewRGBColor(255, 255, 255),
		MinIntensity: 0.0,
		Cycles:       1,
	}
}

type pulseFrame struct {
	PulseFrameParams
	defaultAttr term.Attributes
}

func (s *pulseFrame) Shade(frame, total int, in [][]term.Cell) {
	if total <= 0 || frame < 0 || frame >= total {
		return
	}
	if len(in) == 0 {
		return
	}

	cycles := s.Cycles
	if cycles < 1 {
		cycles = 1
	}
	minI := clamp(s.MinIntensity, 0.0, 1.0)

	// triangleWave maps phase in [0, 1) to a [0, 1, 0] ramp so the pulse
	// rises and falls symmetrically without needing math.Sin.
	phase := fract(float(frame) / float(total) * float(cycles))
	tri := 1.0 - abs(2.0*phase-1.0)
	intensity := minI + (1.0-minI)*smoothstep(0.0, 1.0, tri)
	if intensity <= 0 {
		return
	}

	for y, row := range in {
		for x, cell := range row {
			if !s.isFrameChar(cell.Ch) {
				continue
			}
			in[y][x].Fg = shaderutils.InterpolateColor(
				intensity, cell.Fg, s.Color, s.defaultAttr.Fg,
			)
		}
	}
}

func (s *pulseFrame) isFrameChar(ch rune) bool {
	if ch == 0 {
		return false
	}
	fc := s.FrameCharSet
	return ch == fc.HorizontalTop ||
		ch == fc.HorizontalBottom ||
		ch == fc.VerticalLeft ||
		ch == fc.VerticalRight ||
		ch == fc.TopLeft ||
		ch == fc.TopRight ||
		ch == fc.BottomLeft ||
		ch == fc.BottomRight
}
