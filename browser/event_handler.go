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
}

func (e eventHandlerToHandler) Draw(term.Writer) {
}

func (e eventHandlerToHandler) Handle(ev term.Event) (exit, handled bool) {
	handled = true
	exit = e.h.Handle(ev)
	return
}

func (e eventHandlerToHandler) Cursor() (pos term.Coordinates, show bool) {
	return
}

func (e eventHandlerToHandler) Man() tui.Manual {
	return tui.Manual{}
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

func (e *clientEventHandler) OnUnmount() error {
	e.c.forceCloseHandler(e.handlerID, "clientEventHandler.OnUnmount")
	return nil
}

func (e *clientEventHandler) setHandlerID(handlerID uint32) {
	e.handlerID = handlerID
}

func (e *clientEventHandler) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = e.Handler.Handle(ev)
	if exit {
		e.c.forceCloseHandler(e.handlerID, "clientEventHandler.Handle()(exit=true)")
	}
	return
}

func (h serverEventHandler) Handle(ev term.Event) (exit bool) {
	exit, _ = h.h.Handle(ev)
	if exit {
		h.s.forceCloseHandler(h.handlerID, "serverEventHandler.Handle()(exit=true)")
	}
	return
}

// HandlerEventHandler wraps a tui.Handler to comform to EventHandler.
func HandlerEventHandler(h tui.Handler) EventHandler {
	return handlerToEventHandler{h}
}
