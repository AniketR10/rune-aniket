package proto

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

// NewDrawResponse converts a tui.Component into a DrawResponse.
func NewDrawResponse(comp tui.Component, width, height int) *DrawResponse {
	w := cell.NewBufferWriter(width, height)
	comp.Draw(w)

	resp := new(DrawResponse)
	for _, row := range w.RawCells() {
		var protoRow []*Cell
		for _, c := range row {
			protoRow = append(protoRow, &Cell{
				Character:  uint32(c.Ch),
				Foreground: &Attribute{Flags: uint32(c.Fg)},
				Background: &Attribute{Flags: uint32(c.Bg)},
			})
		}
		resp.Rows = append(resp.Rows, &CellRow{Cells: protoRow})
	}
	return resp
}

// TODO

// SetCell satisfies tui.Cursor
func (r *DrawResponse) SetCell(term.Coordinates, term.Cell) {
}

// Flush satisfies tui.Cursor
func (r *DrawResponse) Flush() error {
	return nil
}

// Clear satisfies tui.Cursor
func (r *DrawResponse) Clear(term.Attributes) error {
	return nil
}

// SetCursor satisfies tui.Cursor
func (r *DrawResponse) SetCursor(term.Coordinates) {
}
