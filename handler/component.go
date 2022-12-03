package handler

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

type withComponent struct {
	tui.Handler
	c tui.Component
}

// WithComponent wraps h with comp for the methods of h that satisfy tui.Component
// and delegates the remaining tui.Handler methods to h.
func WithComponent(h tui.Handler, comp tui.Component) tui.Handler {
	return withComponent{Handler: h, c: comp}
}

func (c withComponent) Draw(w term.Writer) {
	c.c.Draw(w)
}

func (c withComponent) Resize(width, height int) {
	c.c.Resize(width, height)
}
