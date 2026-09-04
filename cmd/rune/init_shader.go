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

package main

import (
	_ "embed"
	"time"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/component/shader"
	"unstable.build/rune/component/shader/glslshader"
)

const (
	initShaderFPS     = 30
	shutdownShaderFPS = 30
)

const initShaderDuration = 3 * time.Second

func initShader(defaultAttr term.Attributes, fc component.FrameCharSet) shader.Shader {
	params := glslshader.BurningPresetGentle(logo, false, fc)
	params.Intensity = []float64{0.2, 0.9, 1.1, 1.1, 0.9, 0.0}
	params.Colors = glslshader.BurningColors{
		HeatColorGradientOverride: []term.Color{
			term.ColorMaroon,
			term.ColorRed,
			term.ColorYellow,
		},
		RipplesGradientStart: []term.Color{
			term.ColorMaroon,
			term.ColorRed,
			term.ColorOlive,
			term.ColorYellow,
			term.ColorYellow,
			term.ColorOlive,
			term.ColorRed,
			term.ColorMaroon,
		},
		RipplesGradientEnd: []term.Color{
			term.ColorMaroon,
			term.ColorRed,
			term.ColorOlive,
			term.ColorYellow,
			term.ColorYellow,
			term.ColorOlive,
			term.ColorRed,
			term.ColorMaroon,
		},
	}
	return glslshader.Burning(params, defaultAttr, initShaderDuration, initShaderFPS)
}
