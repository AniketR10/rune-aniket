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
	"unstable.build/go-tui/term"
)

type nopHandler struct {
	c tui.Component
}

// Nop wraps a tui.Component with a tui.Handler that does nothing.
func Nop(c tui.Component) tui.Handler {
	return nopHandler{c: c}
}

func (n nopHandler) Resize(width, height int) {
	n.c.Resize(width, height)
}

func (n nopHandler) Draw(w term.Writer) {
	n.c.Draw(w)
}

func (n nopHandler) Handle(term.Event) (exit, handled bool) {
	return
}

func (n nopHandler) Cursor() (pos term.Coordinates, style term.CursorStyle, show bool) {
	return
}

func (n nopHandler) Selection() (string, bool) {
	return "", false
}

func (n nopHandler) Man() tui.Manual {
	return tui.Manual{}
}
