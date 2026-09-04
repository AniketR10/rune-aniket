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

package ide

import (
	"time"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/rune/browser"
	"unstable.build/rune/component/shader"
	"unstable.build/rune/component/shader/glslshader"
	"unstable.build/rune/component/shader/timeshader"
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
