package api

import (
	"context"
	"io"

	"unstable.build/go-tui"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/handler"
)

// Handler adds Close to a tui.Handler. Close must be idempotent.
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
	SetContent(Handler) error
	// Focus returns whether this window is in focus.
	Focus() (bool, error)

	// Close closes the window.
	Close() error
}

// Orientation represents a window orientation.
type Orientation uint8

const (
	OrientationDefault Orientation = iota
	OrientationTop
	OrientationBottom
	OrientationLeft
	OrientationRight
)

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
	Split(Orientation, Window, Handler) (Window, error)

	// Floating creates a new floating window at coordinates,
	// with static width and height.
	Floating(h Floating, cfg component.FloatingConfig) (Window, error)

	// Bar creates a status bar with Orientation and Handler.
	// Bars differ from Split and Floating windows in that they can't
	// be in focus and can only receive mouse events.
	Bar(Orientation, tui.Handler) error

	// Tab creates a new tab with h and returns a handle that can be
	// used with the rest of methods that take a browser.Handler.
	// URI is used to uniquely identify a tab and name is used as a label
	// to display it in the tab bar.
	Tab(uri workspaceapi.URI, name string, h Handler) (Handler, error)
}

// Notifications is the interface that wraps methods to display
// messages to the user.
type Notifications interface {
	// Notify delivers the given message to the user, as a notification.
	Notify(level notifications.Level, msg string, args ...interface{}) error

	// NotifyOnce delivers the given notification once, and never again. A notification
	// is identified as the hash of the final message (with arguments).
	NotifyOnce(level notifications.Level, msg string, args ...interface{}) error
}

// ResourceOpener is the interface that wraps the method Open.
type ResourceOpener interface {
	Open(resource workspaceapi.URI) (Handler, error)
}

// EventPublisher is the interface that wraps the method Interrupt.
type EventPublisher interface {
	// Interrupt will publish an interrupt event, which will force
	// redrawing all components in the terminal.
	Interrupt(context.Context) error

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
	Notifications
	io.Closer
}
