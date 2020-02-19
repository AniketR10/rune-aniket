package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

// TestComponent draws rune Ch, and attributes Bg, Fg on every cell
// available. This component is used for testing or debugging.
type TestComponent struct {
	Ch            rune
	Bg, Fg        term.Attribute
	width, height int
}

func (t *TestComponent) Resize(width, height int) {
	t.width, t.height = width, height
}

func (t *TestComponent) Draw(w tui.Writer) {
	for tx := t.width - 1; tx >= 0; tx-- {
		for ty := 0 + t.height - 1; ty >= 0; ty-- {
			w.SetCell(term.Coordinates{X: tx, Y: ty},
				term.Cell{Ch: t.Ch, Fg: t.Fg, Bg: t.Bg})
		}
	}
}
