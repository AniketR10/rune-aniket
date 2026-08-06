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

package gui

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/unstablebuild/rune-go-sdk/term"
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

	// accumY is the accumulated fractional vertical wheel offset. Ebiten
	// reports wheel deltas in (possibly fractional) line units; high-resolution
	// devices such as trackpads emit many small deltas per physical gesture. We
	// accumulate them and emit one discrete wheel event per whole line crossed,
	// keeping the remainder for the next frame. This mirrors Alacritty's
	// accumulated_scroll approach and produces smooth, high-resolution scrolling
	// instead of one jump per frame.
	accumY     float64
	multiplier float64
}

type mouseManager interface {
	Wheel() (float64, float64)
	CursorPosition() (int, int)
	IsMouseButtonPressed(ebiten.MouseButton) bool
}

// defaultScrollMultiplier matches Alacritty's default scrolling.multiplier.
const defaultScrollMultiplier = 3

// maxWheelLinesPerFrame caps how many discrete wheel events a single frame can
// emit. Ebiten reports small per-frame deltas, so a frame that would cross more
// lines than this can only come from a pathological delta (e.g. a malfunctioning
// device or a non-finite value). Capping keeps the slice allocation bounded and
// avoids a makeslice panic without affecting normal scrolling.
const maxWheelLinesPerFrame = 1024

func newMouse(fontManager *font.Manager) *mouse {
	return &mouse{
		fontManager: fontManager,
		mouse:       ebitenInputManager{},
		multiplier:  defaultScrollMultiplier,
	}
}

// processMouse polls the mouse state and returns the events produced since the
// last call. A single frame may yield multiple events: button transitions
// produce at most one event, while wheel movement can emit several discrete
// wheel events depending on the accumulated scroll distance.
func (m *mouse) processMouse() []term.Event {
	state := m.state
	m.state.left = m.mouse.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	m.state.right = m.mouse.IsMouseButtonPressed(ebiten.MouseButtonRight)
	m.state.middle = m.mouse.IsMouseButtonPressed(ebiten.MouseButtonMiddle)
	m.state.x, m.state.y = m.mouse.CursorPosition()
	_, wheelY := m.mouse.Wheel()

	if m.state == state && wheelY == 0 {
		return nil
	}

	pos := m.clampedCoordinates()

	// Wheel events are accumulated and emitted as discrete line events,
	// preserving the fractional remainder for the next frame.
	if wheelY != 0 {
		return m.wheelEvents(pos, wheelY)
	}

	ev := term.Event{Type: term.EventMouse, MouseX: pos.X, MouseY: pos.Y}

	if m.state.left {
		ev.Key = term.MouseLeft
		return []term.Event{ev}
	}

	if m.state.right {
		ev.Key = term.MouseRight
		return []term.Event{ev}
	}

	if m.state.middle {
		ev.Key = term.MouseMiddle
		return []term.Event{ev}
	}

	if (state.left && !m.state.left) ||
		(state.right && !m.state.right) ||
		(state.middle && !m.state.middle) {
		ev.Key = term.MouseRelease
		return []term.Event{ev}
	}

	if m.state.x != state.x || m.state.y != state.y {
		return []term.Event{ev}
	}

	return nil
}

// wheelEvents accumulates the fractional wheel delta and returns one discrete
// wheel event per whole line crossed. The remainder below one line is retained
// for the next frame so high-resolution devices scroll smoothly instead of
// jumping a full line per frame.
func (m *mouse) wheelEvents(pos term.Coordinates, wheelY float64) []term.Event {
	// A non-finite delta would poison the accumulator permanently (NaN
	// propagates, Inf overflows the line count). Drop it and recover the
	// accumulator if a prior frame already poisoned it.
	if !isFinite(wheelY) {
		if !isFinite(m.accumY) {
			m.accumY = 0
		}
		return nil
	}

	m.accumY += wheelY * m.multiplier
	if !isFinite(m.accumY) {
		m.accumY = 0
		return nil
	}

	lines := int(m.accumY)
	m.accumY -= float64(lines)
	if lines == 0 {
		return nil
	}

	key := term.MouseWheelUp
	if lines < 0 {
		key = term.MouseWheelDown
		lines = -lines
	}

	if lines > maxWheelLinesPerFrame {
		lines = maxWheelLinesPerFrame
	}

	events := make([]term.Event, lines)
	for i := range events {
		events[i] = term.Event{
			Type:   term.EventMouse,
			Key:    key,
			MouseX: pos.X,
			MouseY: pos.Y,
		}
	}
	return events
}

func isFinite(f float64) bool {
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

// clampedCoordinates returns the current cursor position in cell coordinates,
// clamped to the bounds of the window.
func (m *mouse) clampedCoordinates() term.Coordinates {
	return m.clamp(m.calculateCoordinates())
}

// clamp bounds a cell position to the window.
func (m *mouse) clamp(pos term.Coordinates) term.Coordinates {
	if pos.X >= m.width {
		pos.X = m.width - 1
	}
	if pos.Y >= m.height {
		pos.Y = m.height - 1
	}
	if pos.X < 0 {
		pos.X = 0
	}
	if pos.Y < 0 {
		pos.Y = 0
	}
	return pos
}

func (m *mouse) calculateCoordinates() (ret term.Coordinates) {
	return m.cellAt(float64(m.state.x), float64(m.state.y))
}

// cellAt converts a pixel position to unclamped cell coordinates.
func (m *mouse) cellAt(x, y float64) (ret term.Coordinates) {
	ret.X = int(m.fontManager.CellX(x))
	ret.Y = int(m.fontManager.CellY(y))
	return
}

func (m *mouse) resize(width, height int) {
	m.width = width
	m.height = height
}
