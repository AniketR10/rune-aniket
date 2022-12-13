package browser

import (
	"context"
	"io"

	"unstable.build/go-tui"
	browserapi "unstable.build/go-tui/api/browser"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
)

// Handler adds Close to a tui.Handler.
type Handler interface {
	tui.Handler
	Close() error
}

// Floating is a Handler used for Floating windows.
// See handler.Floating for more details.
type Floating interface {
	Handler
	handler.Floating
}

// Window is the interface that represents
// a closeable window in a WindowManager.
type Window interface {
	// SetContent sets the content of this window to the given handler.
	SetContent(Handler) error

	// Focus returns whether this window is in focus.
	Focus() (bool, error)

	// Close closes the window. This method is idempotent.
	Close(context.Context) error

	// Content returns the content of this window.
	Content() (Handler, error)

	// OnWindowClosed can be used to install on close callbacks.
	OnWindowClosed(fn func(context.Context))

	// ID is the window identifier.
	ID() uint64

	// Closed returns true if this window has already been closed.
	Closed() bool
}

// WindowManager is the interface that groups tile
// window management methods.
type WindowManager interface {
	// Focus returns the current Window in focus.
	Focus() (Window, error)

	// SetFocus sets win to be the Window in focus and returns the
	// previous window in focus.
	SetFocus(win Window) (Window, error)

	// Split splits the current window in focus in two, and installs
	// Handler in the new window.
	Split(browserapi.Orientation, Window, Handler) (Window, error)

	// Floating creates a new floating window at coordinates,
	// with static width and height.
	Floating(h Floating, cfg component.FloatingConfig) (Window, error)

	// Bar creates a status bar with Orientation and Handler.
	// Bars differ from Split and Floating windows in that they can't
	// be in focus and can only receive mouse events.
	Bar(browserapi.Orientation, tui.Handler) error

	// Tab creates a new tab with h and returns a handle that can be
	// used with the rest of methods that take a browser.Handler.
	// URI is used to uniquely identify a tab and name is used as a label
	// to display it in the tab bar.
	Tab(uri workspaceapi.URI, name string, h Handler) (Handler, error)

	// Window returns a window with the given window ID or returns false
	// if now window with that ID exists.
	Window(uint64) (Window, bool)
}

// Messenger is the interface that wraps methods to display
// messages to the user.
type Messenger interface {
	SetMessage(msg string, args ...interface{}) error
}

// ResourceOpener is the interface that wraps the method Open.
type ResourceOpener interface {
	Open(resource workspaceapi.URI) (Handler, error)
	Resource(workspaceapi.URI) (Handler, bool)
}

// EventPublisher is the interface that wraps the method Interrupt.
type EventPublisher interface {
	// Interrupt will publish an interrupt event, which will force
	// redrawing all components in the terminal.
	Interrupt() error

	// PublishEventNone will publish an EventNone event, which will force
	// calling Handle on the component currently in focus.
	//
	// There's no guarantee that caller will be the tui.Handler that will
	// receive this event.
	PublishEventNone() error
}

// Browser is an interface that groups methods to manipulate
// the user interface of a browser.
type Browser interface {
	WindowManager
	EventPublisher
	ResourceOpener
	Messenger
	io.Closer
}

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
