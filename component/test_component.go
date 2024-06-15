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
package component

import (
	"unstable.build/go-tui/term"
)

var _ WithAttributes = (*TestComponent)(nil)

// TestComponent draws rune Ch, and attributes Bg, Fg on every cell
// available. This component is used for testing or debugging.
type TestComponent struct {
	Ch rune
	term.Attributes
	width, height int
}

func (t *TestComponent) Resize(width, height int) {
	t.width, t.height = width, height
}

func (t *TestComponent) Draw(w term.Writer) {
	for tx := t.width - 1; tx >= 0; tx-- {
		for ty := 0 + t.height - 1; ty >= 0; ty-- {
			w.SetCell(term.Coordinates{X: tx, Y: ty},
				term.Cell{Width: 1, Ch: t.Ch, Attributes: t.Attributes})
		}
	}
}

var _ Responsive = (*TestResponsive)(nil)
var _ Floating = (*TestResponsive)(nil)

// SetAttr satisfies WithAttributes
func (t *TestComponent) SetAttr(attr term.Attributes) (ret term.Attributes) {
	ret = t.Attributes
	t.Attributes = attr
	return
}

type TestResponsive struct {
	TestComponent
	PassedWidth int
	WantHeight  int
	WantWidth   int
}

func (t *TestResponsive) Height(width int) int {
	t.PassedWidth = width
	return t.WantHeight
}

func (t *TestResponsive) Dimensions() (width, height int) {
	return t.WantWidth, t.WantHeight
}
