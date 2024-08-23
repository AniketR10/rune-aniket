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

package handler

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

// TestHandler is a handler used to test composite handlers. Each event
// processed by this handler increments the Ch rune to the next rune.
type TestHandler struct {
	component.TestComponent
	tui.Manual
	CursorPos      term.Coordinates
	CursorStyle    term.CursorStyle
	Exit           bool
	Handled        bool
	HandleOverride func(term.Event) (bool, bool)
	SelectionText  string
}

// NewTestHandler will allocate storage for a new handler and initialize it
func NewTestHandler() (t *TestHandler) {
	t = new(TestHandler)
	t.Ch = 'A'
	return
}

// Handle the next Event
func (t *TestHandler) Handle(ev term.Event) (bool, bool) {
	if t.HandleOverride != nil {
		return t.HandleOverride(ev)
	}
	// signal that we handled the event
	t.Ch++
	return t.Exit, t.Handled
}

// Cursor returns the set CursorPos.
func (t *TestHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	if t.CursorPos == (term.Coordinates{}) {
		return term.Coordinates{X: -1, Y: -1}, 0, false
	}
	return t.CursorPos, t.CursorStyle, true
}

// Selection satisfies tui.Handler.
func (t *TestHandler) Selection() (string, bool) {
	return t.SelectionText, t.SelectionText != ""
}

// Man returns set Manual.
func (t *TestHandler) Man() tui.Manual {
	return t.Manual
}
