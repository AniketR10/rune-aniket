// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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

package main

import (
	_ "embed"
	"time"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/component/shader"
	"unstable.build/go-tui/component/shader/glslshader"
)

const initShaderFPS = 30

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
