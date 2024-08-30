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
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

var _ tui.Component = (*Container)(nil)

// Container provides means to position components in a viewport,
// through a set of Rows that fill the available width and
// stack on top of each other. See Row for more details.
//
// A zero-valued container is ready for use.
type Container struct {
	rows          []*Row
	width, height int
	offset        int
}

// NewContainer allocates storage for a new container and initializes it.
func NewContainer() *Container {
	ret := new(Container)
	return ret
}

// AddRow appends a row to this Container.
func (c *Container) AddRow() *Row {
	row := NewRow()
	c.rows = append(c.rows, row)
	return row
}

// Resize satisfies tui.Component.
func (c *Container) Resize(width, height int) {
	c.width, c.height = width, height
	offset := -c.offset
	for _, row := range c.rows {
		rowHeight := row.Height(width)
		pos := term.Coordinates{Y: offset}
		row.Move(pos)
		if offset == height {
			// move above is enough to ensure that
			// virtual writer does not draw anything
			continue
		}
		if offset+rowHeight >= height {
			rowHeight = height - offset
		}
		// do not resize if it's before start
		if offset+rowHeight >= 0 && rowHeight > 0 {
			row.Resize(width, rowHeight)
		}
		offset += rowHeight
	}
}

// Draw satisfies tui.Component.
func (c *Container) Draw(w term.Writer) {
	// Height requirements could have changed
	// and that's something that we cannot determine from here
	// so it's better to always resize.
	c.Resize(c.width, c.height)

	// provides additional SetCell clipping for components past height
	vw := VirtualWriter{w, term.Coordinates{}, c.height, c.width}
	for _, row := range c.rows {
		row.Draw(vw)
	}
}

// ScrollUp scrolls the contents of this container up.
func (c *Container) ScrollUp() bool {
	if c.offset == 0 {
		return false
	}
	c.offset--
	return true
}

// ScrollDown scrolls the contents of this container down.
func (c *Container) ScrollDown() bool {
	if c.offset == c.maxOffset() {
		return false
	}
	c.offset++
	return true
}

// Height satisfies component.Responsive.
func (c *Container) Height(width int) (height int) {
	for _, row := range c.rows {
		height += row.Height(width)
	}
	return
}

// Dimensions satisfies component.Floating. If underlying
// components do not satisfy component.Floating, then this method panics.
func (r *Container) Dimensions() (retWidth int, retHeight int) {
	for _, row := range r.rows {
		width, height := row.Dimensions()
		if width > retWidth {
			retWidth = width
		}
		retHeight += height
	}
	return
}

func (c *Container) maxOffset() (ret int) {
	for _, row := range c.rows {
		rowHeight := row.Height(c.width)
		ret += rowHeight
	}
	if ret > 0 {
		ret--
	}
	return
}
