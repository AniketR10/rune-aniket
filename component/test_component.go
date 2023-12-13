package component

import (
	"unstable.build/go-tui/term"
)

var _ WithAttributes = (*TestComponent)(nil)

// TestComponent draws rune Ch, and attributes Bg, Fg on every cell
// available. This component is used for testing or debugging.
type TestComponent struct {
	Ch rune
	term.Attributes
	width, height int
}

func (t *TestComponent) Resize(width, height int) {
	t.width, t.height = width, height
}

func (t *TestComponent) Draw(w term.Writer) {
	for tx := t.width - 1; tx >= 0; tx-- {
		for ty := 0 + t.height - 1; ty >= 0; ty-- {
			w.SetCell(term.Coordinates{X: tx, Y: ty},
				term.Cell{Ch: t.Ch, Fg: t.Fg, Bg: t.Bg})
		}
	}
}

var _ Responsive = (*TestResponsive)(nil)
var _ Floating = (*TestResponsive)(nil)

// SetAttr satisfies WithAttributes
func (t *TestComponent) SetAttr(attr term.Attributes) (ret term.Attributes) {
	ret = t.Attributes
	t.Attributes = attr
	return
}

type TestResponsive struct {
	TestComponent
	PassedWidth int
	WantHeight  int
	WantWidth   int
}

func (t *TestResponsive) Height(width int) int {
	t.PassedWidth = width
	return t.WantHeight
}

func (t *TestResponsive) Dimensions() (width, height int) {
	return t.WantWidth, t.WantHeight
}
