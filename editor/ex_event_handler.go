package editor

import (
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/term"
)

type exEventHandler struct {
	e *Ex
	h browser.EventHandler
}

func (h exEventHandler) Handle(ev term.Event) (exit bool) {
	exit = h.h.Handle(ev)
	if exit {
		h.e.unsubscribe(ev)
	}
	return
}

type exEditorEventHandler struct {
	e *Ex
	h EventHandler
}

func (h exEditorEventHandler) Handle(ev Event) (exit bool) {
	exit = h.h.Handle(ev)
	if exit {
		h.e.unsubscribeEditor(h)
	}
	return
}
