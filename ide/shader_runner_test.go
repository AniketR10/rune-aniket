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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/component/shader"
	"unstable.build/rune/component/shader/glslshader"
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
