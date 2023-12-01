package component

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

// Background represents a background Component. See WithBackground.
type Background struct {
	root          tui.Component
	width, height int
	cell          term.Cell
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
func (b *Background) Init(comp tui.Component, cell term.Cell) {
	b.root = comp
	b.cell = cell
}

// Resize : tui.Component
func (b *Background) Resize(width, height int) {
	b.width = width
	b.height = height
	b.root.Resize(width, height)
}

// Draw : tui.Component
func (b *Background) Draw(w term.Writer) {
	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			w.SetCell(term.Coordinates{X: x, Y: y}, b.cell)
		}
	}
	b.root.Draw(w)
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
