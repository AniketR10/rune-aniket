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
	"math/rand/v2"
	"time"

	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/component/shader"
	"unstable.build/go-tui/component/shader/glslshader"
)

const shutdownShaderDuration = 10000 * time.Millisecond

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
