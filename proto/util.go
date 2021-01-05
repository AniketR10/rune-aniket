package proto

import (
	"strings"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

var zeroCell = Cell{}

// NewDrawResponse converts a tui.Component into a DrawResponse.
func NewDrawResponse(comp tui.Component, width, height int) *DrawResponse {
	resp := &DrawResponse{
		Cursor: &DrawResponse_Cursor{
			Position: &Coordinates{},
		},
	}
	w := newDrawResponseWriter(width, height, resp)
	comp.Draw(w)
	return resp
}

type drawResponseWriter struct {
	width, height int
	res           *DrawResponse
}

func newDrawResponseWriter(width, height int, r *DrawResponse) drawResponseWriter {
	cellRowSlab := make([]CellRow, height)
	cellRowWidthSlab := make([]*Cell, height*width)
	r.Rows = make([]*CellRow, height)
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
		width:  width,
		height: height,
		res:    r,
	}
}

// SetCell satisfies term.Writer
func (r drawResponseWriter) SetCell(pos term.Coordinates, c term.Cell) {
	if pos.Y >= r.height || pos.X >= r.width || pos.X < 0 || pos.Y < 0 {
		return
	}

	var cell *Cell
	if r.res.Rows[pos.Y].Cells[pos.X] == &zeroCell {
		cell = new(Cell)
	} else {
		cell = r.res.Rows[pos.Y].Cells[pos.X]
	}
	cell.Character = uint32(c.Ch)
	cell.Foreground = uint32(c.Fg)
	cell.Background = uint32(c.Bg)

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
		}
		if y+1 != len(r.GetRows()) {
			_, _ = builder.WriteRune('\n')
		}
	}
	str = builder.String()
	return
}
