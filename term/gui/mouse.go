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
package gui

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/term/gui/font"
)

type mouseState struct {
	left   bool
	right  bool
	middle bool
	x, y   int
}

type mouse struct {
	fontManager *font.Manager
	mouse       mouseManager

	state         mouseState
	width, height int
}

type mouseManager interface {
	Wheel() (float64, float64)
	CursorPosition() (int, int)
	IsMouseButtonPressed(ebiten.MouseButton) bool
}

func newMouse(fontManager *font.Manager) *mouse {
	return &mouse{fontManager: fontManager, mouse: ebitenInputManager{}}
}

func (m *mouse) processMouse() (ev term.Event, ok bool) {
	state := m.state
	m.state.left = m.mouse.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	m.state.right = m.mouse.IsMouseButtonPressed(ebiten.MouseButtonRight)
	m.state.middle = m.mouse.IsMouseButtonPressed(ebiten.MouseButtonMiddle)
	m.state.x, m.state.y = m.mouse.CursorPosition()
	_, wheelOffset := m.mouse.Wheel()

	ok = m.state != state || wheelOffset != 0
	if !ok {
		return
	}

	ev = term.Event{Type: term.EventMouse}
	pos := m.calculateCoordinates()
	ev.MouseX = pos.X
	ev.MouseY = pos.Y

	if wheelOffset < 0 {
		ev.Key = term.MouseWheelDown
		return
	}

	if wheelOffset > 0 {
		ev.Key = term.MouseWheelUp
		return
	}

	if m.state.left {
		ev.Key = term.MouseLeft
		return
	}

	if m.state.right {
		ev.Key = term.MouseRight
		return
	}

	if m.state.middle {
		ev.Key = term.MouseMiddle
		return
	}

	if (state.left && !m.state.left) ||
		(state.right && !m.state.right) ||
		(state.middle && !m.state.middle) {
		ev.Key = term.MouseRelease
		return
	}

	ok = m.state.x != state.x || m.state.y != state.y
	return
}

func (m *mouse) calculateCoordinates() (ret term.Coordinates) {
	ret.X = int(math.Max(
		0,
		math.Min(
			float64(m.width)-1,
			math.Floor(float64(m.state.x)/m.fontManager.CharSize().X),
		),
	))
	ret.Y = int(math.Max(
		0,
		math.Min(
			float64(m.height)-1,
			math.Floor(float64(m.state.y)/m.fontManager.CharSize().Y),
		),
	))
	return
}

func (m *mouse) resize(width, height int) {
	m.width = width
	m.height = height
}
