package editor

import (
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/term"
)

type browserEventHandler struct {
	browser *Handler
	h       browser.EventHandler
}

func (h browserEventHandler) Handle(ev term.Event) (exit bool) {
	exit = h.h.Handle(ev)
	if exit {
		h.browser.unsubscribe(ev)
	}
	return
}
