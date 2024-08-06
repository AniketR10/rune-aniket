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

// Virtual wraps another tui.Handler to provide cursor event coordinates.
type Virtual struct {
	component.Virtual
}

// Handle tui.Handler
func (v *Virtual) Handle(ev term.Event) (bool, bool) {
	if ev.Type == term.EventMouse {
		offset := v.Position()
		ev.MouseX -= offset.X
		ev.MouseY -= offset.Y
	}
	return v.C.(tui.Handler).Handle(ev)
}

// Cursor tui.Handler
func (v *Virtual) Cursor() (pos term.Coordinates, style term.CursorStyle, show bool) {
	pos, style, show = v.C.(tui.Handler).Cursor()
	offset := v.Position()
	pos.X += offset.X
	pos.Y += offset.Y
	return
}

// Man tui.Handler
func (v *Virtual) Man() tui.Manual {
	return v.C.(tui.Handler).Man()
}
