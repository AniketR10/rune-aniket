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
