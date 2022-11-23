package command

// Dispatcher abstracts the ability to dispatch commands.
type Dispatcher interface {
	Dispatch(cmd string, args ...string) bool
}

// FuncDispatcher returns a Dispatcher that calls fn every time Dispatch is called.
func FuncDispatcher(fn func(string, ...string) bool) Dispatcher {
	return fnDispatcher{fn: fn}
}

type fnDispatcher struct {
	fn func(string, ...string) bool
}

func (d fnDispatcher) Dispatch(cmd string, args ...string) bool {
	return d.fn(cmd, args...)
}
