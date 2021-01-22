package editor

import "github.com/ernestrc/go-tui/browser"

//go:generate mockgen -destination=./event_handler_gomock.go -package editor -self_package editor -source event_handler.go

// EventHandler wraps the basic method Handle.
type EventHandler interface {
	Handle(Event) bool
}

type fnEventHandler struct {
	cb func(Event) bool
}

func (f fnEventHandler) Handle(ev Event) bool {
	return f.cb(ev)
}

// CallbackEventHandler returns an EventHandler that calls fn
// every time Handle is invoked.
func CallbackEventHandler(fn func(Event) bool) EventHandler {
	return fnEventHandler{
		cb: fn,
	}
}

// helps map real Handlers with token.Handler
type serverEventHandler struct {
	s      *Server
	client *eventHandlerClient
}

func (s serverEventHandler) Handle(ev Event) bool {
	brokerID, ok := s.s.nameToID[ev.ResourceName]
	if !ok {
		s.s.tryLog("(%p editor.Server): could NOT dispatch event: handler with resource name %s not found",
			s.s, ev.ResourceName)
		return false
	}
	ev.Resource = browser.Token{ID: uint64(brokerID)}

	// cleaning up upon exit=true is performed via quitCallback
	// of eventHandlerClient so there's no need to check for exit here.
	return s.client.Handle(ev)
}
