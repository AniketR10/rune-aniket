package fractal

import (
	"termbox"
)

type TestComponent struct {
	Ch                  rune
	Bg, Fg              termbox.Attribute
	x, y, width, height int
}

func (t *TestComponent) Resize(width, height int) (err error) {
	t.width, t.height = width, height
	return nil
}

func (t *TestComponent) Move(x, y int) error {
	t.x, t.y = x, y
	return nil
}

func (t *TestComponent) Draw(w Writer) (err error) {
	for tx := t.x + t.width - 1; tx >= t.x; tx-- {
		for ty := t.y + t.height - 1; ty >= t.y; ty-- {
			w.Write(tx, ty, t.Ch, t.Fg, t.Bg)
		}
	}
	return nil
}

func (t *TestComponent) Height() int {
	return t.height
}

func (t *TestComponent) Width() int {
	return t.width
}

func (t *TestComponent) Position() (int, int) {
	return t.x, t.y
}
