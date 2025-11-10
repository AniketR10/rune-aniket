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
	"context"

	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

// Virtual wraps a component to
// provide virtual coordinates and write bound checking.
// It exposes Move which can be used to move the inner component
// in the virtual coordinate space.
type Virtual[T tui.Component] struct {
	C  T
	vw VirtualWriter
	w  term.Writer // prebox, save allocs
}

// Resize resizes the underlying component and stores size
// to perform bound checking on Draw.
func (c *Virtual[T]) Resize(width, height int) {
	c.C.Resize(width, height)
	c.vw.Height = height
	c.vw.Width = width
}

// Draw uses a virtual writer to perform bound checking and
// if successful draw the inner component in the virtual coordinate space.
func (c *Virtual[T]) Draw(writer term.Writer) {
	if c.w == nil {
		c.w = &c.vw
	}
	c.vw.Writer = writer
	c.C.Draw(c.w)
}

// Move changes the position of this virtual component
// in the virtual coordinate space.
func (c *Virtual[T]) Move(pos term.Coordinates) {
	c.vw.Offset = pos
}

// Width returns the width set in last Resize.
func (c *Virtual[T]) Width() int {
	return c.vw.Width
}

// Height returns the height set in last Resize.
func (c *Virtual[T]) Height() int {
	return c.vw.Height
}

// Position returns this virtual component's position
// in the virtual coordinate space.
func (c *Virtual[T]) Position() term.Coordinates {
	return c.vw.Offset
}

var _ term.Writer = (*VirtualWriter)(nil)

// VirtualWriter wraps the given w with a writer that applies an
// offset and SetCell clipping according to offset, height and width.
type VirtualWriter struct {
	Writer        term.Writer
	Offset        term.Coordinates
	Height, Width int
}

// SetCell satisfies term.Writer.
func (w *VirtualWriter) SetCell(pos term.Coordinates, c term.Cell) {
	if pos.X >= w.Width || pos.Y >= w.Height || pos.Y < 0 || pos.X < 0 {
		return
	}
	pos = term.Coordinates{X: w.Offset.X + pos.X, Y: w.Offset.Y + pos.Y}
	w.Writer.SetCell(pos, c)
}

// UnionAttributes satisfies term.Writer.
func (w *VirtualWriter) UnionAttributes(pos term.Coordinates, attr term.Attributes) {
	if pos.X >= w.Width || pos.Y >= w.Height || pos.Y < 0 || pos.X < 0 {
		return
	}
	pos = term.Coordinates{X: w.Offset.X + pos.X, Y: w.Offset.Y + pos.Y}
	w.Writer.UnionAttributes(pos, attr)
}

// Context satisfies term.Writer.
func (w *VirtualWriter) Context() context.Context {
	return w.Writer.Context()
}
