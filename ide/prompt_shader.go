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

package ide

import (
	"time"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component/shader"
	"unstable.build/go-tui/component/shader/glslshader"
	"unstable.build/go-tui/component/shader/timeshader"
)

const (
	promptShaderFPS      = 30
	promptShaderDuration = time.Minute
	promptShaderLoop     = 1200 * time.Millisecond
)

type drawFunc func(term.Writer)

func (f drawFunc) Draw(w term.Writer) { f(w) }
func (drawFunc) Resize(int, int)      {}

func newPromptShader(
	charSet component.FrameCharSet, frameAttr term.Attributes,
	win browser.Window, offset term.Coordinates,
	root tui.Component, interrupter term.Interrupter,
	cfg commandPromptShaderConfig,
) *shader.Component {
	params := glslshader.DefaultRadarFrameParams(charSet)
	if cfg.colorSet {
		params.Color = cfg.color
	}
	if cfg.angularWidth > 0 {
		params.AngularWidth = cfg.angularWidth
	}
	if cfg.cycles > 0 {
		params.Cycles = cfg.cycles
	}
	inner := glslshader.RadarFrame(params, frameAttr)
	cycles := int(promptShaderDuration / promptShaderLoop)
	virt := dynamicVirtual(timeshader.Loop(inner, cycles), win, offset)
	return shader.New(
		root, virt, interrupter, promptShaderFPS, promptShaderDuration,
	)
}

type dynamicVirtualShader struct {
	inner  shader.Shader
	win    browser.Window
	offset term.Coordinates
}

func dynamicVirtual(
	inner shader.Shader, win browser.Window, offset term.Coordinates,
) shader.Shader {
	return &dynamicVirtualShader{inner: inner, win: win, offset: offset}
}

func (s *dynamicVirtualShader) Shade(frame, total int, cells [][]term.Cell) {
	if s.win == nil || s.win.Closed() {
		return
	}
	width, height := s.win.Width(), s.win.Height()
	if width <= 0 || height <= 0 {
		return
	}
	pos := s.win.Position()
	pos.X += s.offset.X
	pos.Y += s.offset.Y
	if pos.X < 0 || pos.Y < 0 || pos.Y >= len(cells) {
		return
	}
	maxY := min(pos.Y+height, len(cells))
	if maxY <= pos.Y {
		return
	}
	view := make([][]term.Cell, 0, maxY-pos.Y)
	for y := pos.Y; y < maxY; y++ {
		row := cells[y]
		if pos.X >= len(row) {
			view = append(view, nil)
			continue
		}
		end := min(pos.X+width, len(row))
		view = append(view, row[pos.X:end])
	}
	s.inner.Shade(frame, total, view)
}
