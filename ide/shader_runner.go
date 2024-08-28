// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/unstablebuild/blue/iterator"
	"unstable.build/go-tui"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/component/shader"
	"unstable.build/go-tui/component/shader/glslshader"
	"unstable.build/go-tui/component/shader/timeshader"
	"unstable.build/go-tui/term"
)

const (
	defaultShaderFPS      = 30
	defaultShaderDuration = 1 * time.Second
)

// used as the root tui.Handler to dynamically run shaders
type shaderRunner struct {
	tui.Handler
	interrupter   term.Interrupter
	shader        *shader.Component
	width, height int
}

func (r *shaderRunner) HandleCommand(ctx context.Context, cmd textapi.Command) (
	err error,
) {
	if len(cmd.Args) == 0 {
		err = errors.New("expected at least one argument with the shader name")
		return
	}

	d := defaultShaderDuration
	if len(cmd.Args) > 1 {
		d, err = time.ParseDuration(cmd.Args[1])
		if err != nil {
			err = fmt.Errorf("parse duration: %v", err)
			return
		}
	}

	fps := defaultShaderFPS
	if len(cmd.Args) > 2 {
		fps, err = strconv.Atoi(cmd.Args[2])
		if err != nil {
			err = fmt.Errorf("parse fps: %v", err)
			return
		}
	}

	var s shader.Shader
	// parse shader and duration
	switch cmd.Args[0] {
	case "blaze":
		s = glslshader.Blaze(glslshader.DefaultBlazeParams(), float64(fps))
	case "blazeBlue":
		params := glslshader.DefaultBlazeParams()
		params.SwapRedBlue = true
		s = glslshader.Blaze(params, float64(fps))
	case "bomb":
		s = shader.Bomb(shader.DefaultBombParams())
	case "embers":
		s = glslshader.Embers(glslshader.DefaultEmbersParams(), float64(fps))
	case "fade":
		s = shader.Fade()
	case "incendium":
		s = glslshader.Incendium(glslshader.DefaultIncendiumParams(), float64(fps))
	case "incendiumReversed":
		s = timeshader.Reverse(glslshader.Incendium(glslshader.DefaultIncendiumParams(), float64(fps)))
	case "incendiumPingPong":
		s = timeshader.PingPong(glslshader.Incendium(glslshader.DefaultIncendiumParams(), float64(fps)))
	case "flames":
		s = glslshader.Flames(glslshader.DefaultFlamesParams(), float64(fps))
	case "flamesA":
		s = glslshader.Flames(glslshader.FlamesPresetAShape(), float64(fps))
	case "flamesV":
		s = glslshader.Flames(glslshader.FlamesPresetVShape(), float64(fps))
	case "inferno":
		s = glslshader.Inferno(glslshader.DefaultInfernoParams(), float64(fps))
	case "infernoBlue":
		params := glslshader.DefaultInfernoParams()
		params.SwapRedBlue = true
		s = glslshader.Inferno(params, float64(fps))
	case "noise":
		s = glslshader.Noise(glslshader.DefaultNoiseParams(), float64(fps))
	case "nop":
		s = shader.Nop()
	case "risingChars":
		s = glslshader.RisingChars(glslshader.DefaultRisingCharsParams())
	case "trippy":
		s = glslshader.Trippy(glslshader.DefaultTrippyParams(), float64(fps))
	default:
		err = errors.New("expected one of the available shaders")
		return
	}

	r.runShader(s, fps, d)
	return
}

func (r *shaderRunner) Complete(ctx context.Context, name string, args []string) (
	iterator.Iterator[string], string, error,
) {
	if len(args) <= 1 {
		return iterator.FromSlice([]string{
			"blaze",
			"blazeBlue",
			"bomb",
			"embers",
			"fade",
			"incendium",
			"incendiumReversed",
			"incendiumPingPong",
			"flames",
			"flamesA",
			"flamesV",
			"inferno",
			"infernoBlue",
			"noise",
			"nop",
			"risingChars",
			"trippy",
		}), "", nil
	}
	return iterator.Empty[string](), "", nil
}

func (r *shaderRunner) init(root tui.Handler, interrupter term.Interrupter) {
	r.Handler = root
	r.interrupter = interrupter
	// initialize zero shader so we can treat field always as non-nil
	r.shader = shader.New(r.Handler, shader.Nop(), r.interrupter, defaultShaderFPS, 0)
}

func (r *shaderRunner) runShader(s shader.Shader, fps int, duration time.Duration) {
	_ = r.shader.Close()
	r.shader = shader.New(r.Handler, s, r.interrupter, fps, duration)
	r.shader.Resize(r.width, r.height)
}

func (r *shaderRunner) Draw(w term.Writer) {
	r.shader.Draw(w)
}

func (r *shaderRunner) Resize(width, height int) {
	r.width = width
	r.height = height
	r.shader.Resize(width, height)
}

func (r *shaderRunner) Close() error {
	return r.shader.Close()
}
