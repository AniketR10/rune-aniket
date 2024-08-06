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

// Floating handlers are not in principle confined to a predetermined
// space and so are allowed certain degree of freedom.
// See component.Floating for more details.
type Floating interface {
	tui.Handler
	component.Floating
}

// StaticFloating wraps a tui.Handler and returns a Floating that always
// return the same Dimensions values.
func StaticFloating(h tui.Handler, width, height int) Floating {
	return staticFloating{width: width, height: height, Handler: h}
}

type staticFloating struct {
	tui.Handler
	width, height int
}

func (s staticFloating) Dimensions() (int, int) {
	return s.width, s.height
}

// PaddedFloating wraps a Floating component and adds a pre-determined
// amount of x axis and y axis padding.
func PaddedFloating(f Floating, padx, pady int) Floating {
	return paddedFloating{padx: padx, pady: pady, Floating: f}
}

// NopFloatingHandler wraps a component.Floating and returns a Floating that does
// nothing when any of the tui.Handler methods are called.
func NopFloatingHandler(h component.Floating) Floating {
	return nopFloating{Floating: h}
}

type paddedFloating struct {
	Floating
	padx, pady int
}

func (p paddedFloating) Dimensions() (width, height int) {
	width, height = p.Floating.Dimensions()
	width += p.padx
	height += p.pady
	return
}

type nopFloating struct {
	component.Floating
}

func (n nopFloating) Handle(term.Event) (bool, bool) {
	return false, false
}

func (n nopFloating) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, term.CursorStyleDefault, false
}

func (n nopFloating) Man() tui.Manual {
	return tui.Manual{}
}
