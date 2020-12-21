package browser

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

// the following structures are helpers to adapt a handler.Client
// to be used as a client and/or server of EventHandler.

// adapts an EventHandler to be used as a WindowHandler
type eventHandlerToHandler struct {
	h EventHandler
}

type handlerToEventHandler struct {
	h tui.Handler
}

type serverEventHandler struct {
	handlerID uint32
	h         tui.Handler
	s         *Server
}

type clientEventHandler struct {
	tui.Handler
	handlerID uint32
	c         *Client
}

func (e eventHandlerToHandler) Resize(width, height int) {
	panic("EventHandler cannot Resize")
}

func (e eventHandlerToHandler) Draw(term.Writer) {
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

func (h handlerToEventHandler) Handle(ev term.Event) bool {
	exit, _ := h.h.Handle(ev)
	return exit
}

func newClientEventHandler(c *Client, h EventHandler) *clientEventHandler {
	return &clientEventHandler{
		Handler: eventHandlerToHandler{h: h},
		c:       c,
	}
}

func (e *clientEventHandler) setHandlerID(handlerID uint32) {
	e.handlerID = handlerID
}

func (e *clientEventHandler) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = e.Handler.Handle(ev)
	if exit {
		go e.c.forceClose(e.handlerID)
	}
	return
}

func (h serverEventHandler) Handle(ev term.Event) (exit bool) {
	exit, _ = h.h.Handle(ev)
	if exit {
		go h.s.forceClose(h.handlerID)
	}
	return
}

// HandlerEventHandler wraps a tui.Handler to comform to EventHandler.
func HandlerEventHandler(h tui.Handler) EventHandler {
	return handlerToEventHandler{h}
}
