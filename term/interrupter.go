package term

// Interrupter wraps the basic method Interrupt, which
// sends an interrupt event to the main loop, forcing a redraw
// of all compontents.
type Interrupter interface {
	Interrupt() error
}

// NopInterrupter is an interrupter that does nothing when Interrupt
// is called.
func NopInterrupter() Interrupter {
	return FuncInterrupter(func() error { return nil })
}

// FuncInterrupter returns an Interrupter that calls fn
// every time Interrupt is called.
func FuncInterrupter(fn func() error) Interrupter {
	return fnInterrupter{fn: fn}
}

type fnInterrupter struct {
	fn func() error
}

func (i fnInterrupter) Interrupt() error {
	return i.fn()
}
