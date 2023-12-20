package component

import (
	"context"

	"unstable.build/go-tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
)

// Background represents a background Component. It wraps a tui.Component
// to make sure that all cells are reset to cell, before comp is drawn.
//
// If cell.Ch is non-zero, then the Background overrides the underlying
// component's attributes in calls to SetCell such that cell.Bg and
// cell.Fg are always used.
type Background struct {
	root          tui.Component
	width, height int
	cell          term.Cell
	buffer        *cell.BufferWriter
}

var _ Floating = (*Background)(nil)
var _ Responsive = (*Background)(nil)
var _ WithAttributes = (*Background)(nil)

// WithBackground is deprecated. Use NewBackground instead.
func WithBackground(comp tui.Component, cell term.Cell) *Background {
	return NewBackground(comp, cell)
}

// NewBackground allocates storage for a new Background and initializes it.
func NewBackground(comp tui.Component, cell term.Cell) *Background {
	ret := new(Background)
	ret.Init(comp, cell)
	return ret
}

// Init initializes this Background with comp and cell.
func (b *Background) Init(comp tui.Component, c term.Cell) {
	b.root = comp
	b.cell = c
	b.buffer = cell.NewBufferWriter(context.Background(), 0, 0)
}

// Resize satisfies tui.Component
func (b *Background) Resize(width, height int) {
	b.width = width
	b.height = height
	b.root.Resize(width, height)
}

// Draw satisfies tui.Component
func (b *Background) Draw(w term.Writer) {
	if b.cell.Ch == 0 {
		for y := 0; y < b.height; y++ {
			for x := 0; x < b.width; x++ {
				w.SetCell(term.Coordinates{X: x, Y: y}, b.cell)
			}
		}
		b.root.Draw(w)
		return
	}

	b.buffer.Init(w.Context(), b.width, b.height)
	b.root.Draw(b.buffer)
	cells := b.buffer.RawCells()

	b.setCells(w, cells)
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
	s.cell.Bg = attr.Bg
	s.cell.Fg = attr.Fg
	return s.root.(WithAttributes).SetAttr(attr)
}

// Height satisfies Responsive if underlying tui.Component
// satisfies Responsive, or panics if it doesn't.
func (s *Background) Height(width int) int {
	return s.root.(Responsive).Height(width)
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
