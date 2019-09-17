package fractal

import (
	"termbox"
)

type TestComponent struct {
	Ch            rune
	Bg, Fg        termbox.Attribute
	width, height int
}

func (t *TestComponent) Resize(width, height int) {
	t.width, t.height = width, height
}

func (t *TestComponent) Draw(w Writer) (err error) {
	for tx := t.width - 1; tx >= 0; tx-- {
		for ty := 0 + t.height - 1; ty >= 0; ty-- {
			w.Write(tx, ty, t.Ch, t.Fg, t.Bg)
		}
	}
	return nil
}
