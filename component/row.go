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

var _ Responsive = (*Row)(nil)

const (
	// MaxCols is the maximum number of available columns in a row.
	MaxCols = 12
)

// Row provides means to horizontally stack a set of components.
// It exposes AddComponent which enables clients to preemptively divide
// the available width in 12 columns. A component that's installed
// with 6 columns for instance, will use exactly 50% of the available width.
//
// Row satisfies component.Responsive by returning the biggest height
// of any of its children.
//
// A zero-valued row is ready to be used.
type Row struct {
	content       []*rowComp
	height, width int
	dirty         bool
	Virtual
}

type rowComp struct {
	cols int
	*Virtual
}

// NewRow allocates storage for a new Row and initializes it.
func NewRow() *Row {
	ret := new(Row)
	return ret
}

// AddComponent adds a component to this Row with the given columns
// over MaxCols as the pre-defined width.
func (r *Row) AddComponent(c Responsive, cols int) *Virtual {
	virt := new(Virtual)
	virt.C = c
	r.content = append(r.content, &rowComp{
		cols:    cols,
		Virtual: virt,
	})
	r.dirty = true
	return virt
}

// Draw satisfies tui.Component.
func (r *Row) Draw(w term.Writer) {
	if r.dirty {
		r.Resize(r.width, r.height)
	}

	// use Virtual position, set by Container
	w = VirtualWriter(w, r.Position(), r.height, r.width)
	for _, comp := range r.content {
		comp.Virtual.Draw(w)
	}
}

// Resize satisfies tui.Component.
func (r *Row) Resize(width, height int) {
	r.width, r.height = width, height
	colWidth := rowColWidth(width)
	var offset int
	for _, comp := range r.content {
		compWidth := rowCompWidth(colWidth, comp)
		pos := term.Coordinates{X: offset}
		comp.Virtual.Move(pos)
		if offset == width {
			continue
		}
		if offset+compWidth >= width {
			compWidth = width - offset
		}
		comp.Virtual.Resize(compWidth, height)
		offset += compWidth
	}
	r.dirty = false
}

// Height satisfies component.Responsive.
func (r *Row) Height(width int) (ret int) {
	colWidth := rowColWidth(width)
	for _, content := range r.content {
		compWidth := rowCompWidth(colWidth, content)
		height := content.Virtual.C.(Responsive).Height(compWidth)
		if height > ret {
			ret = height
		}
	}
	return
}

// Dimensions satisfies component.Floating. If underlying
// components do not satisfy component.Floating, then this method panics.
func (r *Row) Dimensions() (retWidth int, retHeight int) {
	for _, content := range r.content {
		width, height := content.Virtual.C.(Floating).Dimensions()
		if height > retHeight {
			retHeight = height
		}
		retWidth += width
	}
	return
}

func rowColWidth(width int) float64 {
	return float64(width) / float64(MaxCols)
}

func rowCompWidth(colWidth float64, content *rowComp) int {
	return int(float64(content.cols) * colWidth)
}
