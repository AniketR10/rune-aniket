package api

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
)

// FuncHandler returns a Handler by wrapping a tui.Handler
// with an Close callback.
func FuncHandler(h tui.Handler, doClose func() error) Handler {
	return &closeHandler{Handler: h, doClose: doClose}
}

// NopHandler returns a Handler by wrapping a tui.Handler
// with an nop Close callback.
func NopHandler(h tui.Handler) Handler {
	return &closeHandler{Handler: h, doClose: func() error { return nil }}
}

// StaticFloating wraps a Handler and returns a Floating that always
// return the same Dimensions values.
func StaticFloating(h Handler, width, height int) Floating {
	return staticFloating{width: width, height: height, Handler: h}
}

// NopFloatingHandler wraps a handler.Floating and returns a Floating
// that does nothing when Close is called.
func NopFloatingHandler(h handler.Floating) Floating {
	return nopFloating{Floating: h}
}

// FuncFloatingHandler wraps a handler.Floating and returns a Floating
// that calls calls closeFn when Close is called.
func FuncFloatingHandler(h handler.Floating, closeFn func() error) Floating {
	return funcFloatingHandler{Floating: h, fn: closeFn}
}

// FuncFloating wraps a Handler and returns a Floating that
// calls dimFn when Dimensions is called.
func FuncFloating(h Handler, dimFn func() (int, int)) Floating {
	return funcFloating{Handler: h, fn: dimFn}
}

type funcFloatingHandler struct {
	handler.Floating
	fn func() error
}

func (f funcFloatingHandler) Close() error {
	return f.fn()
}

type funcFloating struct {
	Handler
	fn func() (int, int)
}

func (f funcFloating) Dimensions() (width, height int) {
	return f.fn()
}

type closeHandler struct {
	tui.Handler
	doClose func() error
}

func (h *closeHandler) Close() error {
	return h.doClose()
}

type fnEventHandler func(term.Event) bool

// Handle satisfies EventHandler
func (h fnEventHandler) Handle(ev term.Event) bool {
	return h(ev)
}

type staticFloating struct {
	Handler
	width, height int
}

func (s staticFloating) Dimensions() (int, int) {
	return s.width, s.height
}

type nopFloating struct {
	handler.Floating
}

func (n nopFloating) Close() error {
	return nil

}
