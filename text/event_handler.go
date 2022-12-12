package text

import (
	"context"
	textapi "unstable.build/go-tui/api/text"
)

// EventHandler wraps the basic method Handle.
type EventHandler interface {
	// Handle handles Event and returns true if it no longer needs to receive events,
	// in other words it returns true if it's done processing events.
	Handle(context.Context, textapi.Event) bool
}

type fnEventHandler struct {
	cb func(context.Context, textapi.Event) bool
}

func (f fnEventHandler) Handle(ctx context.Context, ev textapi.Event) bool {
	return f.cb(ctx, ev)
}

// FuncEventHandler returns an EventHandler that calls fn
// every time Handle is invoked.
func FuncEventHandler(fn func(context.Context, textapi.Event) bool) EventHandler {
	return fnEventHandler{
		cb: fn,
	}
}
