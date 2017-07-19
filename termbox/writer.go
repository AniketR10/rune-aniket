package termbox

import (
	"github.com/ernestrc/fractal"
	term "github.com/nsf/termbox-go"
)

type TermboxWriter struct {
	bg term.Attribute
	fg term.Attribute
}

func (w *TermboxWriter) SetCell(x, y int, r rune, fg, bg fractal.Attribute) error {
	term.SetCell(x, y, r, term.Attribute(fg), term.Attribute(bg))
	return nil
}
