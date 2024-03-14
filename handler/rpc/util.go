package rpc

import (
	"context"
	"strings"

	"github.com/ernestrc/tcell/v3"
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
	termpb "unstable.build/go-tui/term/rpc"
)

var zeroCell = termpb.Cell{}

// NewDrawResponse converts a tui.Component into a DrawResponse.
func NewDrawResponse(ctx context.Context, comp tui.Component, width, height int) *DrawResponse {
	resp := &DrawResponse{
		Cursor: &DrawResponse_Cursor{
			Position: &termpb.Coordinates{},
		},
	}
	w := newDrawResponseWriter(ctx, width, height, resp)
	comp.Draw(w)
	return resp
}

// DrawResponseToTermString renders a DrawResponse  into str, with width and
// height dimensions.
func DrawResponseToTermString(r *DrawResponse) (str string, width, height int) {
	var builder strings.Builder

	rows := r.GetRows()
	for y, row := range rows {
		height++
		width = 0
		for _, c := range row.GetCells() {
			width++
			ch := c.GetCharacter()
			if ch == 0 {
				ch = ' '
			}
			_, _ = builder.WriteRune(rune(ch))
			for _, comb := range c.GetCombining() {
				_, _ = builder.WriteRune(rune(comb))
			}
		}
		if y+1 != len(r.GetRows()) {
			_, _ = builder.WriteRune('\n')
		}
	}
	str = builder.String()
	return
}

var _ term.Writer = drawResponseWriter{}

type drawResponseWriter struct {
	width, height int
	res           *DrawResponse
	ctx           context.Context
}

// SetCell satisfies term.Writer
func (r drawResponseWriter) SetCell(pos term.Coordinates, c term.Cell) {
	if pos.Y >= r.height || pos.X >= r.width || pos.X < 0 || pos.Y < 0 {
		return
	}

	var cell *termpb.Cell
	if r.res.Rows[pos.Y].Cells[pos.X] == &zeroCell {
		cell = new(termpb.Cell)
	} else {
		cell = r.res.Rows[pos.Y].Cells[pos.X]
	}
	cell.Character = uint32(c.Ch)
	cell.Foreground = uint64(c.Fg)
	cell.Background = uint64(c.Bg)
	cell.Attrs = int64(c.Attrs)
	cell.Width = uint32(c.Width)
	for _, c := range c.Combining {
		cell.Combining = append(cell.Combining, uint32(c))
	}

	r.res.Rows[pos.Y].Cells[pos.X] = cell
}

func (r drawResponseWriter) UnionAttributes(pos term.Coordinates, attr term.Attributes) {
	if pos.Y >= r.height || pos.X >= r.width || pos.X < 0 || pos.Y < 0 {
		return
	}

	var cell *termpb.Cell
	if r.res.Rows[pos.Y].Cells[pos.X] == &zeroCell {
		cell = new(termpb.Cell)
	} else {
		cell = r.res.Rows[pos.Y].Cells[pos.X]
	}

	uattr := term.AttributesUnion(term.Attributes{
		Fg:    tcell.Color(cell.Foreground),
		Bg:    tcell.Color(cell.Background),
		Attrs: tcell.AttrMask(cell.Attrs),
	}, attr)

	cell.Foreground = uint64(uattr.Fg)
	cell.Background = uint64(uattr.Bg)
	cell.Attrs = int64(uattr.Attrs)

	r.res.Rows[pos.Y].Cells[pos.X] = cell
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

func newDrawResponseWriter(ctx context.Context, width, height int, r *DrawResponse) drawResponseWriter {
	cellRowSlab := make([]termpb.CellRow, height)
	cellRowWidthSlab := make([]*termpb.Cell, height*width)
	r.Rows = make([]*termpb.CellRow, height)
	for i := 0; i < height; i++ {
		r.Rows[i] = &cellRowSlab[i]
		r.Rows[i].Cells = cellRowWidthSlab[i*width : (i+1)*width]
		for j := 0; j < width; j++ {
			// SetCell substitutes zeroCell for a newly allocated cell;
			// this allows us to speed up client/server communication
			r.Rows[i].Cells[j] = &zeroCell
		}
	}
	return drawResponseWriter{
		ctx:    ctx,
		width:  width,
		height: height,
		res:    r,
	}
}
