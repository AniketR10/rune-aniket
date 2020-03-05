package proto

import (
	"strings"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

// NewDrawResponse converts a tui.Component into a DrawResponse.
func NewDrawResponse(comp tui.Component, width, height int) *DrawResponse {
	resp := new(DrawResponse)
	w := newDrawResponseWriter(width, height, resp)
	comp.Draw(w)
	return resp
}

type drawResponseWriter struct {
	width, height int
	res           *DrawResponse
}

func newDrawResponseWriter(width, height int, r *DrawResponse) drawResponseWriter {
	r.Rows = make([]*CellRow, height)
	for i := 0; i < height; i++ {
		r.Rows[i] = &CellRow{Cells: make([]*Cell, width)}
	}
	return drawResponseWriter{
		width:  width,
		height: height,
		res:    r,
	}
}

// SetCell satisfies tui.Cursor
func (r drawResponseWriter) SetCell(pos term.Coordinates, c term.Cell) {
	if pos.Y >= r.height || pos.X >= r.width || pos.X < 0 || pos.Y < 0 {
		return
	}
	r.res.Rows[pos.Y].Cells[pos.X] = &Cell{
		Character:  uint32(c.Ch),
		Foreground: &Attribute{Flags: uint32(c.Fg)},
		Background: &Attribute{Flags: uint32(c.Bg)},
	}
}

// Flush satisfies tui.Cursor
func (r drawResponseWriter) Flush() error {
	return nil
}

// Clear satisfies tui.Cursor
func (r drawResponseWriter) Clear(term.Attributes) error {
	return nil
}

// SetCursor satisfies tui.Cursor
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
