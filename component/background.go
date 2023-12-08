package component

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
)

// Background represents a background Component. See WithBackground.
type Background struct {
	root          tui.Component
	width, height int
	cell          term.Cell
	buffer        *cell.BufferWriter
}

var _ Floating = (*Background)(nil)
var _ Responsive = (*Background)(nil)
var _ WithAttributes = (*Background)(nil)

// WithBackground wraps comp into a Background Component
// which makes sure that all cells are reset to cell, before comp is drawn.
func WithBackground(comp tui.Component, cell term.Cell) *Background {
	ret := new(Background)
	ret.Init(comp, cell)
	return ret
}

// Init initializes this Background with comp and cell.
func (b *Background) Init(comp tui.Component, c term.Cell) {
	b.root = comp
	b.cell = c
	b.buffer = cell.NewBufferWriter(0, 0)
}

// Resize : tui.Component
func (b *Background) Resize(width, height int) {
	b.width = width
	b.height = height
	b.root.Resize(width, height)
}

// Draw : tui.Component
func (b *Background) Draw(w term.Writer) {
	b.buffer.Init(b.width, b.height)
	b.root.Draw(b.buffer)
	cells := b.buffer.RawCells()

	if b.cell.Ch == 0 {
		b.setCellsFast(w, cells)
		return
	}

	b.setCells(w, cells)
}

func (b *Background) setCells(w term.Writer, cells [][]term.Cell) {
	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			cell := b.cell
			if y < len(cells) && x < len(cells[y]) {
				ocell := cells[y][x]
				if ocell.Ch != 0 {
					cell.Ch = ocell.Ch
				}
				if ocell.Fg != 0 {
					cell.Fg = ocell.Fg
				}
				// but no background
			}
			w.SetCell(term.Coordinates{X: x, Y: y}, cell)
		}
	}
}

func (b *Background) setCellsFast(w term.Writer, cells [][]term.Cell) {
	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			cell := b.cell
			if y < len(cells) && x < len(cells[y]) {
				ocell := cells[y][x]
				cell.Ch = ocell.Ch
				cell.Fg = ocell.Fg
				// but no background
			}
			w.SetCell(term.Coordinates{X: x, Y: y}, cell)
		}
	}
}

// Content returns the inner component.
func (b *Background) Content() tui.Component {
	return b.root
}

// Dimensions satisfies Floating if underlying tui.Component
// satisfies Floating, or panics if it doesn't.
func (s *Background) Dimensions() (width, height int) {
	return s.root.(Floating).Dimensions()
}

// SetAttr satisfies WithAttributes if underlying tui.Component
// satisfies WithAttributes, or panics if it doesn't.
func (s *Background) SetAttr(attr term.Attributes) term.Attributes {
	return s.root.(WithAttributes).SetAttr(attr)
}

// Height satisfies Responsive if underlying tui.Component
// satisfies Responsive, or panics if it doesn't.
func (s *Background) Height(width int) int {
	return s.root.(Responsive).Height(width)
}
