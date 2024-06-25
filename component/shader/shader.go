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
package shader

import (
	"context"
	"time"

	"unstable.build/go-tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

// New wraps a tui.Component with the given shader and returns
// a component that will use the give interrupter to animate it
// at the given fps. If fps is 0, a sane default is used.
func New(
	comp tui.Component, shader Shader,
	interrupter term.Interrupter, fps int,
	duration time.Duration,
) tui.Component {
	if fps == 0 {
		fps = defaultFPS
	}
	// here we use Animation just for the FPS implementation
	frame := &shaderFrame{
		buf:    cell.NewBufferWriter(context.Background(), 1, 1),
		shader: shader,
		comp:   comp,
		total:  int(duration / time.Duration(int(time.Second)/fps)),
	}
	frames, sequence := []tui.Component{frame}, []int{0}

	a := new(component.Animation)
	a.InitWithComponents(context.Background(), interrupter, frames, sequence, fps)
	return shaderAnimation{shaderFrame: frame, animation: a, root: comp}
}

// Shader abstracts the ability to aplly shader-like effects
// to other components.
type Shader interface {
	// Shade applies the shader's transformation
	// to the given component turning it into an animated component,
	// or returns false if this Shader's animation is done.
	Shade(frame, total int, in [][]term.Cell)
}

const defaultFPS = 30

type shaderFrame struct {
	buf    *cell.BufferWriter
	shader Shader
	comp   tui.Component
	total  int

	epoch int
	done  bool
}

func (s *shaderFrame) Draw(w term.Writer) {
	s.buf.SetContext(w.Context())
	_ = s.buf.Clear(term.Attributes{})

	s.comp.Draw(s.buf)

	cells := s.buf.RawCells()
	s.shader.Shade(s.epoch, s.total, s.buf.RawCells())

	for y, row := range cells {
		for x, cell := range row {
			w.SetCell(term.Coordinates{Y: y, X: x}, cell)
		}
	}
	s.epoch++
	s.done = s.epoch == s.total
}

func (s *shaderFrame) Resize(width, height int) {
	s.buf = cell.NewBufferWriter(context.Background(), width, height)
	s.comp.Resize(width, height)
}

type shaderAnimation struct {
	root        tui.Component
	animation   *component.Animation
	shaderFrame *shaderFrame
}

func (s shaderAnimation) Draw(w term.Writer) {
	if !s.shaderFrame.done {
		s.animation.Draw(w)
		if s.shaderFrame.done {
			s.animation.Close()
		}
	} else {
		s.root.Draw(w)
	}
}

func (s shaderAnimation) Resize(width, height int) {
	if !s.shaderFrame.done {
		s.animation.Resize(width, height)
	} else {
		s.root.Resize(width, height)
	}
}
