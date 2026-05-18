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
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component/shader"
	"unstable.build/go-tui/component/shader/glslshader"
	"unstable.build/go-tui/component/shader/timeshader"
)

const (
	defaultShaderFPS      = 30
	defaultShaderDuration = 1 * time.Second
	// defaultLoadingShaderDuration is the fallback lifetime of the
	// loading shader's [shader.Component] when the caller does not
	// configure one via [WithLoadingShader]. It must be longer than the
	// shader's animated portion so the Component does not auto-expire
	// mid-load: the loading shader is expected to "hold" at its final
	// visual frame (see [shader.GrayFadeParams.FadeFrames]) until
	// stopLoading explicitly swaps it out. Without this, the
	// [shader.Component] sets done = true after duration elapses and
	// the bare root is drawn — making the screen "gain back color"
	// mid-load and flashing the freshly-installed workspace right
	// before the open shader takes over.
	//
	// 10s is an upper bound on plausible addWorkspace latency; the
	// shader is cancelled cleanly when stopLoading runs anyway.
	defaultLoadingShaderDuration = 10 * time.Second
	// defaultOpenShaderDuration is the fallback lifetime of the open
	// shader's [shader.Component] when the caller does not configure
	// one via [WithOpenShader].
	defaultOpenShaderDuration = 1 * time.Second
)

type shutdownShaderConfig struct {
	shader   func(term.Attributes) shader.Shader
	fps      int
	duration time.Duration
}

func nopShutdownShaderConfig() shutdownShaderConfig {
	return shutdownShaderConfig{
		shader:   func(_ term.Attributes) shader.Shader { return shader.Nop() },
		fps:      defaultShaderFPS,
		duration: 100 * time.Millisecond,
	}
}

// loadingShaderConfig describes the shader played while a
// workspace is loading. If shader is nil, no loading shader is
// played. duration is the lifetime of the underlying
// [shader.Component]; it is intentionally decoupled from the
// visual length of the effect itself (see e.g.
// [shader.GrayFadeParams.FadeFrames]) so callers can configure a
// "fade then hold" budget that outlives the animation. Zero falls
// back to [defaultLoadingShaderDuration].
type loadingShaderConfig struct {
	shader   func(term.Attributes) shader.Shader
	fps      int
	duration time.Duration
}

// openShaderConfig describes the shader played when a workspace
// finishes installing. If shader is nil, no open shader is played.
// duration is the lifetime of the underlying [shader.Component]; zero
// falls back to [defaultOpenShaderDuration].
type openShaderConfig struct {
	shader   func(term.Attributes) shader.Shader
	fps      int
	duration time.Duration
}

// used as the root tui.Handler to dynamically run shaders
type shaderRunner struct {
	tui.Handler
	interrupter       term.Interrupter
	shader            *shader.Component
	defAttr           term.Attributes
	fc                component.FrameCharSet
	width, height     int
	shutdownShaderCfg shutdownShaderConfig
	loadingShaderCfg  loadingShaderConfig
	openShaderCfg     openShaderConfig
	openShaderCells   [][]term.Cell
	// loadingShaderInst holds the live loading-shader instance while it
	// runs, so stopLoading can query its progress (e.g.
	// [shader.GrayFadeShader.Progress]) and forward it to the open shader
	// for visual continuity. Reset to nil once stopLoading runs.
	loadingShaderInst shader.Shader
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

	s, ok := buildNamedShader(cmd.Args[0], r.defAttr, fps, d, r.fc)
	if !ok {
		err = errors.New("expected one of the available shaders")
		return
	}

	r.runShader(s, fps, d)
	return
}

func (r *shaderRunner) Complete(ctx context.Context, cmd textapi.Command) (
	iterator.Iterator[string], string, error,
) {
	if len(cmd.Args) <= 1 {
		return iterator.FromSlice(namedShaderNames()), "", nil
	}
	return iterator.Empty[string](), "", nil
}

// namedShaderNames returns the names accepted by buildNamedShader, in
// the order shown by the :shaderrun completer. The order is preserved
// for showroom UI stability.
func namedShaderNames() []string {
	return []string{
		// TODO: Review and leave only useful shaders. This at the moment is a review showroom!
		"blaze",
		"blazeBlue",
		"bomb",
		"burn",
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
		"burningOnlyRedFlamesLightsOn",
		"burningOnlyRedFlamesLightsOff",
		"burningOnlyBlueFlamesLightsOn",
		"burningOnlyBlueFlamesLightsOff",
		"noise",
		"nop",
		"risingChars",
		"shine",
		"shineFrame",
		"grayFade",
		"trippy",
	}
}

// buildNamedShader returns the shader registered under name with the
// default parameters used by the :shaderrun showroom, wrapped in the
// same cross-fade transitions when applicable. fps and duration are
// only consumed by shaders that need them (GLSL animated shaders and
// the burning preset family). Returns ok=false when name does not
// match any known shader.
func buildNamedShader(
	name string, defAttr term.Attributes,
	fps int, duration time.Duration, fc component.FrameCharSet,
) (shader.Shader, bool) {
	const (
		fadeInPerc  = 0.1
		fadeOutPerc = 0.1
	)
	switch name {
	case "blaze":
		return wrapShaderCrossFadeInOut(
			glslshader.Blaze(glslshader.DefaultBlazeParams(), float64(fps)),
			fadeInPerc, fadeOutPerc,
			defAttr,
		), true
	case "blazeBlue":
		params := glslshader.DefaultBlazeParams()
		params.SwapRedBlue = true
		return wrapShaderCrossFadeInOut(
			glslshader.Blaze(params, float64(fps)),
			fadeInPerc, fadeOutPerc,
			defAttr,
		), true
	case "bomb":
		return shader.Bomb(shader.DefaultBombParams(), defAttr), true
	case "burn":
		return shader.Burn(shader.DefaultBurnParams(), defAttr), true
	case "embers":
		return wrapShaderCrossFadeInOut(
			glslshader.Embers(glslshader.DefaultEmbersParams(), defAttr, float64(fps)),
			fadeInPerc, fadeOutPerc,
			defAttr,
		), true
	case "fade":
		return shader.Fade(defAttr), true
	case "incendium":
		return wrapShaderCrossFadeInOut(
			glslshader.Incendium(glslshader.DefaultIncendiumParams(), defAttr, float64(fps)),
			fadeInPerc, fadeOutPerc,
			defAttr,
		), true
	case "incendiumReversed":
		return wrapShaderCrossFadeInOut(
			timeshader.Reverse(glslshader.Incendium(glslshader.DefaultIncendiumParams(), defAttr, float64(fps))),
			fadeInPerc, fadeOutPerc,
			defAttr,
		), true
	case "incendiumPingPong":
		return wrapShaderCrossFadeInOut(
			timeshader.PingPong(glslshader.Incendium(
				glslshader.DefaultIncendiumParams(), defAttr, float64(fps))),
			fadeInPerc, fadeOutPerc,
			defAttr,
		), true
	case "flames":
		return wrapShaderCrossFadeInOut(
			glslshader.Flames(glslshader.DefaultFlamesParams(), defAttr, float64(fps)),
			0.0, fadeOutPerc,
			defAttr,
		), true
	case "flamesA":
		return wrapShaderCrossFadeInOut(
			glslshader.Flames(glslshader.FlamesPresetAShape(), defAttr, float64(fps)),
			0.0, fadeOutPerc,
			defAttr,
		), true
	case "flamesV":
		return wrapShaderCrossFadeInOut(
			glslshader.Flames(glslshader.FlamesPresetVShape(), defAttr, float64(fps)),
			0.0, fadeOutPerc,
			defAttr,
		), true
	case "inferno":
		return wrapShaderCrossFadeInOut(
			glslshader.Inferno(glslshader.DefaultInfernoParams(), float64(fps)),
			fadeInPerc, fadeOutPerc,
			defAttr,
		), true
	case "infernoBlue":
		params := glslshader.DefaultInfernoParams()
		params.SwapRedBlue = true
		return wrapShaderCrossFadeInOut(
			glslshader.Inferno(params, float64(fps)),
			fadeInPerc, fadeOutPerc,
			defAttr,
		), true
	case "burningOnlyRedFlamesLightsOn":
		params := glslshader.BurningPresetGentleOnlyFlames(false, fc)
		return glslshader.Burning(params, defAttr, duration, float64(fps)), true
	case "burningOnlyRedFlamesLightsOff":
		params := glslshader.BurningPresetGentleOnlyFlames(true, fc)
		return glslshader.Burning(params, defAttr, duration, float64(fps)), true
	case "burningOnlyBlueFlamesLightsOn":
		params := glslshader.BurningPresetGentleOnlyFlames(false, fc)
		params.Colors.SwapRedBlue = true
		return glslshader.Burning(params, defAttr, duration, float64(fps)), true
	case "burningOnlyBlueFlamesLightsOff":
		params := glslshader.BurningPresetGentleOnlyFlames(true, fc)
		params.Colors.SwapRedBlue = true
		return glslshader.Burning(params, defAttr, duration, float64(fps)), true
	case "noise":
		return wrapShaderCrossFadeInOut(
			glslshader.Noise(glslshader.DefaultNoiseParams(), float64(fps)),
			fadeInPerc, fadeOutPerc,
			defAttr,
		), true
	case "nop":
		return shader.Nop(), true
	case "risingChars":
		return wrapShaderCrossFadeInOut(
			glslshader.RisingChars(glslshader.DefaultRisingCharsParams(), defAttr),
			fadeInPerc, fadeOutPerc,
			defAttr,
		), true
	case "shine":
		return wrapShaderCrossFadeInOut(
			glslshader.Shine(glslshader.DefaultShineParams(), defAttr),
			fadeInPerc, fadeOutPerc,
			defAttr,
		), true
	case "shineFrame":
		return wrapShaderCrossFadeInOut(
			glslshader.ShineFrame(glslshader.DefaultShineFrameParams(fc), defAttr),
			fadeInPerc, fadeOutPerc,
			defAttr,
		), true
	case "grayFade":
		return shader.GrayFade(shader.DefaultGrayFadeParams(), defAttr), true
	case "trippy":
		return wrapShaderCrossFadeInOut(
			glslshader.Trippy(glslshader.DefaultTrippyParams(), float64(fps)),
			fadeInPerc, fadeOutPerc,
			defAttr,
		), true
	}
	return nil, false
}

func (r *shaderRunner) init(
	root tui.Handler,
	interrupter term.Interrupter,
	defAttr term.Attributes,
	shutdownShaderCfg shutdownShaderConfig,
	loadingShaderCfg loadingShaderConfig,
	openShaderCfg openShaderConfig,
	fc component.FrameCharSet,
) {
	r.Handler = root
	r.interrupter = interrupter
	r.defAttr = defAttr
	r.shutdownShaderCfg = shutdownShaderCfg
	r.loadingShaderCfg = loadingShaderCfg
	r.openShaderCfg = openShaderCfg
	r.fc = fc

	// Initialize zero shader so we can treat field always as non-nil.
	//
	// Some duration > 0 is passed so the r.shader.done stays true,
	// that's something that happens after rendering any shader and
	// allows us to only use one variable (c.done) to determine if a
	// shader is running or not.
	r.shader = shader.New(r.Handler, shader.Nop(), r.interrupter, defaultShaderFPS, 100*time.Millisecond)
}

func (r *shaderRunner) runShader(s shader.Shader, fps int, duration time.Duration) {
	_ = r.shader.Close()
	r.shader = shader.New(r.Handler, s, r.interrupter, fps, duration)
	r.shader.Resize(r.width, r.height)
}

// captureOpenShaderCells captures the wrapped root as it exists before the
// workspace manager switches focus/slots. The open shader uses this as its
// stable base layer; capturing inside Shader.Shade is too late because the
// root has already drawn the newly opened workspace by then.
func (r *shaderRunner) captureOpenShaderCells() {
	if r.width <= 0 || r.height <= 0 || r.Handler == nil {
		r.openShaderCells = nil
		return
	}
	buf := cell.NewBufferWriter(context.Background(), r.width, r.height)
	_ = buf.Clear(term.Attributes{})
	r.Handler.Draw(buf)
	r.openShaderCells = term.CloneCells(buf.RawCells())
}

func (r *shaderRunner) cancel() {
	// Some duration > 0 is passed so the r.shader.done stays true,
	// that's something that happens after rendering any shader and
	// allows us to only use one variable (c.done) to determine if a
	// shader is running or not.
	r.runShader(shader.Nop(), defaultShaderFPS, 100*time.Millisecond)
}

func (r *shaderRunner) runShutdownShader() {
	// If no shutdown shader is set this will run a shader.Nop(). If a shader was
	// running when runShutdownShader is called it will appear as canceling it.
	r.runShader(
		r.shutdownShaderCfg.shader(r.defAttr),
		r.shutdownShaderCfg.fps,
		r.shutdownShaderCfg.duration)
}

// startLoading is called every time addWorkspace begins loading
// a workspace. It (re)starts the loading shader, cancelling any
// shader currently playing. No-op when no loading shader was
// configured.
func (r *shaderRunner) startLoading() {
	r.openShaderCells = nil
	r.loadingShaderInst = nil
	if r.loadingShaderCfg.shader == nil {
		return
	}
	loading := r.loadingShaderCfg.shader(r.defAttr)
	r.loadingShaderInst = loading
	duration := r.loadingShaderCfg.duration
	if duration <= 0 {
		duration = defaultLoadingShaderDuration
	}
	r.runShader(loading, r.loadingShaderCfg.fps, duration)
}

// stopLoading is called when an addWorkspace lifecycle
// completes. It swaps in the open shader (or cancels back to
// Nop if no open shader was configured). No-op when no loading
// shader was configured.
func (r *shaderRunner) stopLoading() {
	openShaderCells := r.openShaderCells
	r.openShaderCells = nil
	loadingShaderInst := r.loadingShaderInst
	r.loadingShaderInst = nil
	if r.loadingShaderCfg.shader == nil {
		return
	}
	if r.openShaderCfg.shader == nil {
		r.cancel()
		return
	}
	openShader := r.openShaderCfg.shader(r.defAttr)
	if len(openShaderCells) > 0 {
		if sh, ok := openShader.(interface{ SetInitialCells([][]term.Cell) }); ok {
			sh.SetInitialCells(openShaderCells)
		}
	}
	// If the loading shader was the gray-fade desaturation, pre-desaturate
	// the open shader's snapshot by the same amount so the burn starts
	// from the same visual state instead of snapping back to the
	// originally-captured colors. This is what makes the loading→open
	// transition feel like one continuous effect.
	if gf, ok := loadingShaderInst.(*shader.GrayFadeShader); ok {
		if sh, ok := openShader.(interface {
			SetInitialDesaturation(amount float64)
		}); ok {
			sh.SetInitialDesaturation(gf.Progress())
		}
	}
	duration := r.openShaderCfg.duration
	if duration <= 0 {
		duration = defaultOpenShaderDuration
	}
	r.runShader(openShader, r.openShaderCfg.fps, duration)
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

// Adds fade in and out to a given shader.
//
// fadeInPerc specifies the percentage of the whole animation you want fading
// in (e.g. 0.1 means 10% of the beginning frames of shader will be
// transition), and similarly with fadeOutPerc (10% would mean at 90% of the
// animation it starts fading out).
func wrapShaderCrossFadeInOut(
	sh shader.Shader, fadeInPerc, fadeOutPerc float64,
	defaultAttr term.Attributes,
) shader.Shader {
	return shader.TransitionCrossFade(
		shader.TransitionCrossFadeParams{
			ChangeAtPerc: fadeInPerc,
			OverlapPerc:  fadeInPerc,
		},
		defaultAttr,
		shader.Nop(),
		shader.TransitionCrossFade(
			shader.TransitionCrossFadeParams{
				ChangeAtPerc: 1.0 - fadeOutPerc,
				OverlapPerc:  fadeOutPerc,
			},
			defaultAttr,
			sh,
			shader.Nop(),
		),
	)
}
