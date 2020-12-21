package editor

import (
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/term"
)

type exEventHandler struct {
	browser *Ex
	h       browser.EventHandler
}

func (h exEventHandler) Handle(ev term.Event) (exit bool) {
	exit = h.h.Handle(ev)
	if exit {
		h.browser.unsubscribe(ev)
	}
	return
}
