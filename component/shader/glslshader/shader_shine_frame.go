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
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/component/asciiart"
	"unstable.build/go-tui/component/shader"
	"unstable.build/go-tui/component/shader/shaderutils"
)

// ShineFrame produces a diagonal shining effect that sweeps from the bottom
// left of the screen to the top right.
//
// The effect is applied exclusively to cells whose character matches one of
// the runes in the configured [component.FrameCharSet]; every other cell is
// left untouched. As the band moves over a matching cell the cell's
// foreground color is interpolated towards [ShineFrameParams.Color] giving a
// glint impression along the frame strokes.
func ShineFrame(params ShineFrameParams, defaultAttr term.Attributes) shader.Shader {
	return &shineFrame{ShineFrameParams: params, defaultAttr: defaultAttr}
}

// ShineFrameParams allows you to customize the [ShineFrame] effect.
type ShineFrameParams struct {
	// Direction along which the shine band sweeps.
	Direction Direction
	// FrameCharSet is the set of frame characters the shine effect is
	// applied to. Cells whose character is not part of this set are left
	// untouched.
	FrameCharSet component.FrameCharSet
	// Color is the color blended into the foreground of matching cells when
	// the shine band passes over them.
	Color tcell.Color
	// BandWidth controls how wide the shine band is, expressed as a fraction
	// of the (aspect-ratio corrected) diagonal length.
	//
	// (range 0..1 clamped, must be > 0)
	BandWidth float
	// Cycles is the number of times the shine band sweeps across the animation.
	// Values less than 1 are treated as 1.
	Cycles int
}

// DefaultShineFrameParams returns a sane set of [ShineFrameParams] using the
// given [component.FrameCharSet] as the set of cells the shine applies to.
func DefaultShineFrameParams(fc component.FrameCharSet) ShineFrameParams {
	return ShineFrameParams{
		Direction:    DirectionBottomLeftToTopRight,
		FrameCharSet: fc,
		Color:        tcell.NewRGBColor(255, 255, 255),
		BandWidth:    0.25,
		Cycles:       1,
	}
}

type shineFrame struct {
	ShineFrameParams
	defaultAttr term.Attributes
}

func (s *shineFrame) Shade(frame, total int, in [][]term.Cell) {
	if total <= 0 || frame < 0 || frame >= total {
		return
	}
	rows := len(in)
	if rows == 0 {
		return
	}

	cols := 0
	for _, row := range in {
		if len(row) > cols {
			cols = len(row)
		}
	}
	if cols == 0 {
		return
	}

	cycles := s.Cycles
	if cycles < 1 {
		cycles = 1
	}
	bandWidth := clamp(s.BandWidth, 0.0, 1.0)
	if bandWidth <= 0 {
		return
	}

	// Sweep "pulse" from -bandWidth to 1+bandWidth so the band fully enters
	// from the bottom-left corner and fully exits past the top-right corner.
	pulse := fract(float(frame) / float(total) * float(cycles))
	pos := pulse*(1.0+2.0*bandWidth) - bandWidth

	maxX := float(cols - 1)
	if maxX <= 0 {
		maxX = 1
	}
	maxY := float(rows-1) * asciiart.HeightToWidthCellAspectRatio
	if maxY <= 0 {
		maxY = 1
	}

	for y, row := range in {
		// y axis is flipped (terminal y=0 at top) and scaled by the cell
		// aspect ratio so the diagonal looks visually balanced.
		yNorm := (float(rows-1-y) * asciiart.HeightToWidthCellAspectRatio) / maxY
		for x, cell := range row {
			if !s.isFrameChar(cell.Ch) {
				continue
			}
			xNorm := float(x) / maxX
			t := directionT(s.Direction, xNorm, yNorm)
			intensity := smoothstep(bandWidth, 0.0, abs(t-pos))
			if intensity <= 0 {
				continue
			}
			in[y][x].Fg = shaderutils.InterpolateColor(
				intensity, cell.Fg, s.Color, s.defaultAttr.Fg,
			)
		}
	}
}

func (s *shineFrame) isFrameChar(ch rune) bool {
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
