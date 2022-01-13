package editor

import "context"

//go:generate mockgen -destination=./event_handler_gomock.go -package editor -self_package editor -source event_handler.go

// EventHandler wraps the basic method Handle.
type EventHandler interface {
	// Handle handles Event and returns true if it no longer needs to receive events,
	// in other words it returns true if it's done processing events.
	Handle(context.Context, Event) bool
}

type fnEventHandler struct {
	cb func(context.Context, Event) bool
}

func (f fnEventHandler) Handle(ctx context.Context, ev Event) bool {
	return f.cb(ctx, ev)
}

// FuncEventHandler returns an EventHandler that calls fn
// every time Handle is invoked.
func FuncEventHandler(fn func(context.Context, Event) bool) EventHandler {
	return fnEventHandler{
		cb: fn,
	}
}
