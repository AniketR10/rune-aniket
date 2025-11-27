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

// Inline renders the given slice of components next to each other,
// occupying the maximum height of all the components but respecting
// each component's width. If there's overflow, then the overflowing
// components will be truncated, at the opposite side of Alignment,
// will be truncated.
func Inline(components []Floating, alignment Alignment) Floating {
	comps := make([]*Virtual[Floating], len(components))
	for i := range comps {
		comps[i] = &Virtual[Floating]{C: components[i]}
	}
	return inline{alignment: alignment, comps: comps}
}

type inline struct {
	alignment Alignment
	comps     []*Virtual[Floating]
}

func (i inline) Dimensions() (width int, height int) {
	for _, comp := range i.comps {
		cwidth, cheight := comp.C.Dimensions()
		width += cwidth
		if cheight > height {
			height = cheight
		}
	}
	return
}

func (i inline) Resize(width, height int) {
	var offset int
	for _, comp := range i.comps {
		desiredWidth, _ := comp.C.Dimensions()
		if desiredWidth > width {
			desiredWidth = width
		}
		comp.Move(term.Coordinates{X: offset})
		comp.Resize(desiredWidth, height)
		width -= desiredWidth
		offset += desiredWidth
	}
}

func (i inline) Draw(w term.Writer) {
	for _, comp := range i.comps {
		comp.Draw(w)
	}
}
