// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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


package shop

import (
	"time"

	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"

	"unstable.build/go-tui/component/shader"
	"unstable.build/go-tui/component/shader/glslshader"
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
