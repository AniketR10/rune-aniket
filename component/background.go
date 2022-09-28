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
