package handler

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

type nopHandler struct{}

// Nop returns a tui.Handler that does nothing.
func Nop() tui.Handler {
	return nopHandler{}
}

func (n nopHandler) Resize(width, height int) {
}

func (n nopHandler) Draw(tui.Writer) {
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
