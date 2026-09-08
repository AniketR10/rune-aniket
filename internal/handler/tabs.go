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

package handler

import (
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/rune/internal/component"
)

var _ tui.Handler = (*Tabs)(nil)

// Tabs add mouse handling to component.Tabs.
type Tabs struct {
	component.Tabs
	mousePressedLeft bool
	OnClick          func(int) bool
}

// NewTabs returns a Tabs component which handles mouse events.
func NewTabs() *Tabs {
	t := new(Tabs)
	t.Init()
	return t
}

// Init initializes this Tabs with the given underlying handler
// and frame attributes.
func (t *Tabs) Init() {
	t.Tabs.Init()
}

// Handle delegates the event to the underlying handler.
func (t *Tabs) Handle(ev term.Event) (quit, handled bool) {
	defer func() {
		// if button is released then MouseRelease is dispatched
		// so this is reset
		t.mousePressedLeft = ev.Key == term.MouseLeft
	}()

	if ev.Type != term.EventMouse || ev.Key != term.MouseLeft || ev.Mod != 0 {
		return
	}

	// do not dispatch drags as multiple click events:
	// MouseRelease must be dispatched between MouseLeft for
	// events to be considered multiple mouse clicks.
	pressedLeft := ev.Key == term.MouseLeft && !t.mousePressedLeft
	if !pressedLeft {
		return
	}
	mousePos := term.Coordinates{X: ev.MouseX, Y: ev.MouseY}
	idx, ok := t.Tabs.TabAt(mousePos)
	if !ok {
		if t.OnClick != nil {
			handled = t.OnClick(-1)
		}
		return
	}

	handled = true

	t.SetFocus(idx)
	if t.OnClick != nil {
		_ = t.OnClick(idx)
	}
	return
}

// Cursor satisfies tui.Handler but always returns false.
func (t *Tabs) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, term.CursorStyleDefault, false
}

// Selection satisfies tui.Handler but always returns false.
func (t *Tabs) Selection() (string, bool) {
	return "", false
}
