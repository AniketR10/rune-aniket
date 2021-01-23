package editor

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

// FuncEventHandler returns an EventHandler that calls fn
// every time Handle is invoked.
func FuncEventHandler(fn func(Event) bool) EventHandler {
	return fnEventHandler{
		cb: fn,
	}
}
