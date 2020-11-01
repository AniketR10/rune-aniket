package handler

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

type nopHandler struct {
	c tui.Component
}

// Nop wraps a tui.Component with a tui.Handler that does nothing.
func Nop(c tui.Component) tui.Handler {
	return nopHandler{c: c}
}

func (n nopHandler) Resize(width, height int) {
	n.c.Resize(width, height)
}

func (n nopHandler) Draw(w term.Writer) {
	n.c.Draw(w)
}

func (n nopHandler) Handle(term.Event) (exit, handled bool) {
	return
}

func (n nopHandler) Cursor() (pos term.Coordinates, show bool) {
	return
}

func (n nopHandler) Man() tui.Manual {
	return tui.Manual{}
}
