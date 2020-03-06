package browser

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
)

// the following structures are helpers to adapt a handler.Client
// to be used as a client and/or server of EventHandler.

// adapts a handler.Client to be used as a EventHandler
type eventHandler struct {
	cc *handler.Client
}

func (e eventHandler) Handle(ev term.Event) (exit bool) {
	exit, _ = e.cc.Handle(ev)
	return
}

// adapts an EventHandler to be used as a tui.Handler
type eventHandlerToHandler struct {
	h EventHandler
}

func (e eventHandlerToHandler) Resize(width, height int) {
	panic("EventHandler cannot Resize")
}

func (e eventHandlerToHandler) Draw(tui.Writer) {
	panic("EventHandler cannot Draw")
}

func (e eventHandlerToHandler) Handle(ev term.Event) (exit, handled bool) {
	handled = true
	exit = e.h.Handle(ev)
	return
}

func (e eventHandlerToHandler) Cursor() (pos term.Coordinates, show bool) {
	panic("EventHandler cannot Cursor")
}

func (e eventHandlerToHandler) Man() tui.Manual {
	panic("EventHandler cannot Man")
}
