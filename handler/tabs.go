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

var _ tui.Handler = (*Tabs)(nil)

// Tabs add mouse handling to component.Tabs.
type Tabs struct {
	component.Tabs

	OnClick func(int)
}

// NewTabs returns a Tabs component which handles mouse events.
func NewTabs() *Tabs {
	t := new(Tabs)
	t.Init()
	return t
}

// Init initializes this Tabs with the given underlying handler
// and frame attributes.
func (f *Tabs) Init() {
	f.Tabs.Init()
}

// Handle delegates the event to the underlying handler.
func (f *Tabs) Handle(ev term.Event) (quit, handled bool) {
	if ev.Type != term.EventMouse || ev.Key != term.MouseLeft {
		return
	}
	mousePos := term.Coordinates{X: ev.MouseX, Y: ev.MouseY}
	idx, ok := f.Tabs.TabAt(mousePos)
	if !ok {
		return
	}

	handled = true

	f.SetFocus(idx)
	if f.OnClick != nil {
		f.OnClick(idx)
	}
	return
}

// Cursor returns the underlying handler's cursor position
// with the frame offset.
func (f *Tabs) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, term.CursorStyleDefault, false
}

// Man just delegates Man call to underlying handler.
func (f *Tabs) Man() tui.Manual {
	panic("TODO")
}
