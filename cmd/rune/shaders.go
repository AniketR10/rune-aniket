// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2026 Unstable Build, All Rights Reserved.
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
	"math/rand/v2"
	"time"

	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/component/shader"
	"unstable.build/go-tui/component/shader/glslshader"
)

const (
	// loadingShaderFPS is the cadence at which the shader played
	// while a workspace is loading is animated.
	loadingShaderFPS = 30

	// loadingShaderFadeDuration is how long the gray-fade animation takes
	// to reach full desaturation. The loading shader keeps running past
	// this point, holding at full desaturation, until the workspace finishes
	// loading and the open shader takes over.
	loadingShaderFadeDuration = 1 * time.Second

	// loadingShaderDuration is the lifetime of the underlying
	// [shader.Component] hosting the loading shader. It is intentionally
	// longer than loadingShaderFadeDuration so the Component does not
	// auto-expire mid-load: the loading shader holds at full desaturation
	// past the fade until [ide.WithLoadingShader]'s consumer swaps in the
	// open shader. 10s is an upper bound on plausible addWorkspace
	// latency; the shader is cancelled cleanly when the load completes.
	loadingShaderDuration = 10 * time.Second

	shutdownShaderDuration = 10000 * time.Millisecond

	// openShaderFPS is the cadence at which the shader played when a
	// workspace finishes loading is animated.
	openShaderFPS = 60

	// openShaderDuration is the lifetime of the open shader's
	// [shader.Component] — how long the burn sweep runs after the
	// workspace finishes loading.
	openShaderDuration = 1200 * time.Millisecond
)

// loadingShader returns the shader played while a workspace is
// loading. It slowly fades the current screen to gray; the runner
// then swaps in the open shader which starts from the same tint to
// keep the two effects visually continuous. If loading completes
// before the fade does, the runner ends it early and reads
// [shader.GrayFadeShader.Progress] so the open shader matches the
// partial fade state.
func loadingShader(defaultAttr term.Attributes) shader.Shader {
	params := shader.DefaultGrayFadeParams()
	params.FadeFrames = int(loadingShaderFadeDuration / time.Second * loadingShaderFPS)
	return shader.GrayFade(params, defaultAttr)
}

// openShader returns the shader played when a workspace finishes
// loading. A quick "burn" sweep ignites the canvas and resolves
// back to the original content, acknowledging the transition
// without slowing the user down.
func openShader(defaultAttr term.Attributes) shader.Shader {
	params := shader.DefaultBurnParams()
	params.BurnGradient = []tcell.Color{
		tcell.ColorWhite,
		tcell.ColorSilver,
		tcell.ColorYellow,
		tcell.ColorRed,
		tcell.ColorMaroon,
	}
	params.BurnSymbols = []rune{
		'░', '▒', '▓', '█', '█', '▓', '▒', '░',
	}
	params.BurnDuration = 0.25
	params.SmokeChance = 0
	//params.SmokeSymbols = []rune{
	//	'▀', '▐', '▄', '▌',
	//}
	//params.SmokeRise = 0.8
	//params.SmokeMaxRise = 10

	return shader.Burn(params, defaultAttr)
}

func shutdownShader(defaultAttr term.Attributes) shader.Shader {
	params := glslshader.DefaultFlamesParams()
	params.ColorGradient = []tcell.Color{
		tcell.ColorRed,
		tcell.ColorMaroon,
	}

	// randomize some of the parameters
	randomizer := func() float64 { return min(1.1, float64(rand.Float32()+0.4)) }
	params.Belly = -75.0 * randomizer()
	params.Clumps = 2.5 * randomizer()
	params.ClumpHeight = 1.8 * randomizer()
	params.NoiseAmount = 0.8 * randomizer()
	params.NoiseSize = 0.5 * randomizer()
	return glslshader.Flames(params, defaultAttr, 60)
}
