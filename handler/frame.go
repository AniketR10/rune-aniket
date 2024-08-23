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

// Frame is a proxy handler that simply draws a frame around
// the underlying handler.
type Frame struct {
	component.Frame
	handler tui.Handler
}

// NewFrame allocates storage for a new Frame and initializes it.
func NewFrame(handler tui.Handler) (f *Frame) {
	f = new(Frame)
	f.Init(handler)
	return
}

// Init initializes this Frame with the given underlying handler
// and frame attributes.
func (f *Frame) Init(handler tui.Handler) {
	f.handler = handler
	f.Frame.Init(handler)
}

// Handle delegates the event to the underlying handler.
func (f *Frame) Handle(ev term.Event) (bool, bool) {
	if ev.Type == term.EventMouse {
		content := f.Frame.ContentPosition()
		ev.MouseX -= content.X
		ev.MouseY -= content.Y
		if ev.MouseX < 0 {
			ev.MouseX = 0
		}
		if ev.MouseY < 0 {
			ev.MouseY = 0
		}
	}
	return f.handler.Handle(ev)
}

// Cursor returns the underlying handler's cursor position
// with the frame offset.
func (f *Frame) Cursor() (pos term.Coordinates, style term.CursorStyle, show bool) {
	pos, style, show = f.handler.Cursor()
	content := f.Frame.ContentPosition()
	pos.X += content.X
	pos.Y += content.Y
	return
}

// Selection returns the underlying handler's selection.
func (f *Frame) Selection() (string, bool) {
	return f.handler.Selection()
}

// Man just delegates Man call to underlying handler.
func (f *Frame) Man() tui.Manual {
	return f.handler.Man()
}
