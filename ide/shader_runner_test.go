// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/component/shader"
	"unstable.build/go-tui/component/shader/glslshader"
)

// fakeLoadingShader returns a deterministic, cheap shader factory
// usable in tests. We use Shine because it's pure CPU and avoids
// the heavier GL paths.
func fakeLoadingShader() func(term.Attributes) shader.Shader {
	return func(defaultAttr term.Attributes) shader.Shader {
		params := glslshader.DefaultShineParams()
		params.Color = term.NewRGBColor(255, 255, 255)
		return glslshader.Shine(params, defaultAttr)
	}
}

func newTestShaderRunner(loading, open loadingShaderConfig) *shaderRunner {
	r := new(shaderRunner)
	openCfg := openShaderConfig{
		shader:   open.shader,
		fps:      open.fps,
		duration: open.duration,
	}
	r.init(handler.Nop(), term.NopInterrupter(), term.Attributes{},
		nopShutdownShaderConfig(), loading, openCfg,
		component.FrameCharSetDefault())
	return r
}

// TestShaderRunnerLoadingNoOpWithoutConfig verifies that
// startLoading and stopLoading are silent no-ops when no
// loading shader is configured, so callers can issue them
// unconditionally without disturbing the active shader.
func TestShaderRunnerLoadingNoOpWithoutConfig(t *testing.T) {
	r := newTestShaderRunner(loadingShaderConfig{}, loadingShaderConfig{})
	initial := r.shader

	r.startLoading()
	r.startLoading()
	assert.Same(t, initial, r.shader,
		"startLoading must not swap the shader when no loading config is set")

	r.stopLoading()
	r.stopLoading()
	assert.Same(t, initial, r.shader,
		"stopLoading must not swap the shader when no loading config is set")
}

// TestShaderRunnerStartLoadingRestarts verifies that a second
// startLoading cancels and replaces the previous loading
// animation, rather than stacking or being ignored.
func TestShaderRunnerStartLoadingRestarts(t *testing.T) {
	loading := loadingShaderConfig{shader: fakeLoadingShader(), fps: 30}
	r := newTestShaderRunner(loading, loadingShaderConfig{})
	initial := r.shader

	r.startLoading()
	first := r.shader
	assert.NotSame(t, initial, first,
		"first startLoading must swap in a loading shader")

	r.startLoading()
	second := r.shader
	assert.NotSame(t, first, second,
		"second startLoading must replace the previous loading shader")
}

// TestShaderRunnerStopLoadingFiresOpenShader verifies that
// stopLoading swaps in the open shader when configured.
func TestShaderRunnerStopLoadingFiresOpenShader(t *testing.T) {
	loading := loadingShaderConfig{shader: fakeLoadingShader(), fps: 30}
	open := loadingShaderConfig{shader: fakeLoadingShader(), fps: 30}
	r := newTestShaderRunner(loading, open)

	r.startLoading()
	loadingComp := r.shader

	r.stopLoading()
	assert.NotSame(t, loadingComp, r.shader,
		"stopLoading must swap in the open shader")
}

// TestShaderRunnerStopLoadingFallsBackToCancelWithoutOpen
// verifies that stopLoading cancels the active shader when no
// open shader is configured so the loading animation does not
// stay on screen forever.
func TestShaderRunnerStopLoadingFallsBackToCancelWithoutOpen(t *testing.T) {
	loading := loadingShaderConfig{shader: fakeLoadingShader(), fps: 30}
	r := newTestShaderRunner(loading, loadingShaderConfig{})

	r.startLoading()
	loadingComp := r.shader

	r.stopLoading()
	assert.NotSame(t, loadingComp, r.shader,
		"stopLoading without an open shader must cancel the active one")
}

// TestShaderRunnerStartLoadingUsesConfiguredDuration verifies that the
// loading shader's containing shader.Component is sized using the
// duration configured on loadingShaderConfig, so callers can decouple
// the visual fade length from the host-component lifetime instead of
// being bound to a hard-coded shaderRunner constant.
func TestShaderRunnerStartLoadingUsesConfiguredDuration(t *testing.T) {
	const (
		fps    = 30
		dur    = 4 * time.Second
		expect = int(dur) / (int(time.Second) / fps) // shader.Component.total
	)
	loading := loadingShaderConfig{
		shader:   fakeLoadingShader(),
		fps:      fps,
		duration: dur,
	}
	r := newTestShaderRunner(loading, loadingShaderConfig{})

	r.startLoading()
	assert.Equal(t, expect, r.shader.Total(),
		"startLoading must use the configured duration")
}

// TestShaderRunnerStopLoadingUsesConfiguredOpenDuration verifies that
// the open shader's containing shader.Component is sized using the
// duration configured on openShaderConfig.
func TestShaderRunnerStopLoadingUsesConfiguredOpenDuration(t *testing.T) {
	const (
		fps    = 30
		dur    = 2500 * time.Millisecond
		expect = int(dur) / (int(time.Second) / fps)
	)
	loading := loadingShaderConfig{shader: fakeLoadingShader(), fps: fps}
	open := loadingShaderConfig{
		shader:   fakeLoadingShader(),
		fps:      fps,
		duration: dur,
	}
	r := newTestShaderRunner(loading, open)

	r.startLoading()
	r.stopLoading()
	assert.Equal(t, expect, r.shader.Total(),
		"stopLoading must use the configured open-shader duration")
}

// TestBuildNamedShaderRegistersEveryListedName asserts that every name
// returned by namedShaderNames resolves through buildNamedShader.
func TestBuildNamedShaderRegistersEveryListedName(t *testing.T) {
	defAttr := term.Attributes{}
	fc := component.FrameCharSetDefault()
	for _, name := range namedShaderNames() {
		sh, ok := buildNamedShader(name, defAttr, 30, time.Second, fc)
		assert.True(t, ok,
			"buildNamedShader must register %q (listed by namedShaderNames)",
			name)
		assert.NotNil(t, sh,
			"buildNamedShader(%q) must return a non-nil Shader", name)
	}
}
