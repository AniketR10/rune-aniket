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

package starlarktutorial

import (
	"math"
	"time"

	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/go-tui/component/shader"
	"unstable.build/go-tui/component/shader/shaderutils"
)

const hintFPS = 30
const hintDuration = 3 * time.Second

// defaultAttr.Fg resolves cells whose Fg is [term.ColorDefault];
// when both are default, the underlying [shaderutils.InterpolateColor]
// short-circuits to a no-op rather than emitting near-black RGB.
func buildHintPulse(defaultAttr term.Attributes, x, y, width int) shader.Shader {
	return shader.Virtual(
		&textBlink{
			target:       term.ColorGray,
			defaultFg:    defaultAttr.Fg,
			periodFrames: 30,
		},
		term.Coordinates{X: x, Y: y}, width, 1,
	)
}

type textBlink struct {
	target       term.Color
	defaultFg    term.Color
	periodFrames int
}

func (s *textBlink) Shade(frame, _ int, cells [][]term.Cell) {
	if s.periodFrames < 2 || frame < 0 {
		return
	}
	// Sine cycles 0..1..0 over periodFrames; smoother than a
	// triangle wave for a perceptual heartbeat.
	cyclePos := float64(frame%s.periodFrames) / float64(s.periodFrames)
	intensity := math.Sin(math.Pi * cyclePos)
	if intensity <= 0 {
		return
	}
	for y := range cells {
		for x := range cells[y] {
			ch := cells[y][x].Ch
			if ch == 0 || ch == ' ' {
				continue
			}
			cells[y][x].Fg = shaderutils.InterpolateColor(
				intensity, cells[y][x].Fg, s.target, s.defaultFg,
			)
		}
	}
}
