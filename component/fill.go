package component

import (
	"github.com/ernestrc/fractal"
	termbox "github.com/nsf/termbox-go"
)

type Fill struct {
	Ch                  rune
	Bg, Fg              termbox.Attribute
	x, y, width, height int
}

func (t *Fill) Resize(width, height int) (err error) {
	t.width, t.height = width, height
	return nil
}

func (t *Fill) Move(x, y int) error {
	t.x, t.y = x, y
	return nil
}

func (t *Fill) Draw(w fractal.Writer) (err error) {
	for tx := t.x + t.width - 1; tx >= t.x; tx-- {
		for ty := t.y + t.height - 1; ty >= t.y; ty-- {
			w.Write(tx, ty, t.Ch, t.Fg, t.Bg)
		}
	}
	return nil
}

func (t *Fill) Height() int {
	return t.height
}

func (t *Fill) Width() int {
	return t.width
}

func (t *Fill) Position() (int, int) {
	return t.x, t.y
}
