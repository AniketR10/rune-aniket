// Copyright (C) 2017-2026 The Rune Authors
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

// Package glassbar renders a vertical column of native macOS buttons
// floating on top of the cell grid, along the right edge of the window.
// The grid never draws under them: callers reserve a matching column
// with browser.Config.RightInset and keep the bar aligned with it by
// feeding SetFrame the column's rect from gui.GUI.CellRect.
package glassbar

// Button is one native overlay button.
type Button struct {
	// ID is echoed back to the Install activation callback.
	ID string
	// Symbol is the SF Symbol name drawn as the button's image.
	Symbol string
	// Tooltip is the hover help text.
	Tooltip string
}

// Padding is the gap in points between the reserved column's edges and
// the buttons, and between consecutive buttons. Buttons are square and
// as wide as the column allows.
const Padding = 4.0

// Supported reports whether this platform draws the bar at all.
func Supported() bool { return supported }

// frame is the last rect handed to SetFrame, replayed by Install so the
// two can arrive in either order.
var frame struct{ x, y, width float64 }

// Install replaces the bar with buttons and routes clicks to activate,
// which is called with the clicked button's ID. It must run on the main
// thread, after the application window exists.
func Install(buttons []Button, activate func(id string)) {
	install(buttons, activate)
	setFrame(frame.x, frame.y, frame.width)
}

// SetFrame anchors the bar's top-left corner at (x, y) points from the
// top-left of the window's content area and sizes its buttons to fit
// width. It must run on the main thread.
func SetFrame(x, y, width float64) {
	frame.x, frame.y, frame.width = x, y, width
	setFrame(x, y, width)
}
