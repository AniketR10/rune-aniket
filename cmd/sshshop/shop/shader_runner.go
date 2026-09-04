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

package shop

import (
	"time"

	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"

	"unstable.build/rune/component/shader"
	"unstable.build/rune/component/shader/glslshader"
)

const (
	startupShaderFPS      = 30
	startupShaderDuration = 10 * time.Second
)

// shaderRunner mirrors the IDE's root shader wrapper in spirit, but
// only exposes the one behavior we currently need in sshshop: run the
// RisingChars shader over the root handler for the first 10 seconds of a
// session, then fall through to the unmodified root UI.
type shaderRunner struct {
	tui.Handler
	shader *shader.Component
}

func newShaderRunner(root tui.Handler, interrupter term.Interrupter) *shaderRunner {
	sh := glslshader.RisingChars(
		glslshader.DefaultRisingCharsParams(),
		term.Attributes{},
	)
	r := &shaderRunner{Handler: root}
	r.shader = shader.New(root, sh, interrupter, startupShaderFPS, startupShaderDuration)
	return r
}

// NewShaderRoot wraps the given root handler in the shop's startup
// shader runner. The returned handler should be passed directly to
// tui.RunScreen.
func NewShaderRoot(root tui.Handler, interrupter term.Interrupter) *shaderRunner {
	return newShaderRunner(root, interrupter)
}

func (r *shaderRunner) Draw(w term.Writer) {
	r.shader.Draw(w)
}

func (r *shaderRunner) Resize(width, height int) {
	r.shader.Resize(width, height)
}

func (r *shaderRunner) Close() error {
	return r.shader.Close()
}
