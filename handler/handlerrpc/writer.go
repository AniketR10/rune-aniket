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

package handlerrpc

import (
	"context"

	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/term/termrpc"
)

var zeroCell = termrpc.Cell{}

var _ term.Writer = drawResponseWriter{}

// NewDrawResponse converts a tui.Component into a DrawResponse.
func NewDrawResponse(
	ctx context.Context, comp tui.Component, width, height int,
) *DrawStreamResponse {
	resp := &DrawStreamResponse{
		Cursor: &DrawStreamResponse_Cursor{
			Position: &termrpc.Coordinates{},
		},
	}
	w := newDrawResponseWriter(ctx, width, height)
	comp.Draw(w)
	resp.Rows = w.rows
	return resp
}

type drawResponseWriter struct {
	width, height int
	rows          []*termrpc.CellRow
	ctx           context.Context
}

// SetCell satisfies term.Writer
func (r drawResponseWriter) SetCell(pos term.Coordinates, c term.Cell) {
	if pos.Y >= r.height || pos.X >= r.width || pos.X < 0 || pos.Y < 0 {
		return
	}

	var cell *termrpc.Cell
	if r.rows[pos.Y].Cells[pos.X] == &zeroCell {
		cell = new(termrpc.Cell)
	} else {
		cell = r.rows[pos.Y].Cells[pos.X]
	}
	cell.Character = uint32(c.Ch)
	cell.Foreground = uint64(c.Fg)
	cell.Background = uint64(c.Bg)
	cell.Attrs = int64(c.Attrs)
	cell.Width = uint32(c.Width)
	for _, c := range c.Combining {
		cell.Combining = append(cell.Combining, uint32(c))
	}

	r.rows[pos.Y].Cells[pos.X] = cell
}

func (r drawResponseWriter) UnionAttributes(pos term.Coordinates, attr term.Attributes) {
	if pos.Y >= r.height || pos.X >= r.width || pos.X < 0 || pos.Y < 0 {
		return
	}

	var cell *termrpc.Cell
	if r.rows[pos.Y].Cells[pos.X] == &zeroCell {
		cell = new(termrpc.Cell)
	} else {
		cell = r.rows[pos.Y].Cells[pos.X]
	}

	uattr := term.AttributesUnion(term.Attributes{
		Fg:    tcell.Color(cell.Foreground),
		Bg:    tcell.Color(cell.Background),
		Attrs: tcell.AttrMask(cell.Attrs),
	}, attr)

	cell.Foreground = uint64(uattr.Fg)
	cell.Background = uint64(uattr.Bg)
	cell.Attrs = int64(uattr.Attrs)

	r.rows[pos.Y].Cells[pos.X] = cell
}

// Flush satisfies term.Writer
func (r drawResponseWriter) Flush() error {
	return nil
}

// Clear satisfies term.Writer
func (r drawResponseWriter) Clear(term.Attributes) error {
	return nil
}

// SetCursor satisfies term.Writer
func (r drawResponseWriter) SetCursor(term.Coordinates) {
}

func (r drawResponseWriter) Context() context.Context {
	return r.ctx
}

func newDrawResponseWriter(ctx context.Context, width, height int) drawResponseWriter {
	cellRowSlab := make([]termrpc.CellRow, height)
	cellRowWidthSlab := make([]*termrpc.Cell, height*width)
	rows := make([]*termrpc.CellRow, height)
	for i := 0; i < height; i++ {
		rows[i] = &cellRowSlab[i]
		rows[i].Cells = cellRowWidthSlab[i*width : (i+1)*width]
		for j := 0; j < width; j++ {
			// SetCell substitutes zeroCell for a newly allocated cell;
			// this allows us to speed up client/server communication
			rows[i].Cells[j] = &zeroCell
		}
	}
	return drawResponseWriter{
		ctx:    ctx,
		width:  width,
		height: height,
		rows:   rows,
	}
}
