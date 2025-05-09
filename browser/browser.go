// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package browser

import (
	"io"

	"unstable.build/go-tui"
	"unstable.build/go-tui/api/browserapi"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
)

// Floating is a Handler used for Floating windows.
// See handler.Floating for more details.
type Floating interface {
	browserapi.Handler
	handler.Floating
}

// Window is the interface that represents
// a closeable window in a WindowManager.
type Window interface {
	// SetContent sets the content of this window to the given handler.
	SetContent(browserapi.Handler) error

	// Focus returns whether this window is in focus.
	Focus() (bool, error)

	// Close closes the window. This method is idempotent.
	Close() error

	// Content returns the content of this window.
	Content() (browserapi.Handler, error)

	// ID is the window identifier.
	ID() uint64

	// Closed returns true if this window has already been closed.
	Closed() bool

	// IsFloating returns true if window is a floating window,
	// or false if window is a tiled window.
	IsFloating() bool
}

// WindowManager is the interface that groups window and tab management methods.
type WindowManager interface {
	TabManager

	// Focus returns the current Window in focus.
	Focus() (Window, error)

	// SetFocus sets win to be the Window in focus and returns the
	// previous window in focus.
	SetFocus(win Window) (Window, error)

	// Split splits the current window in focus in two, and installs
	// Handler in the new window.
	Split(browserapi.Orientation, Window, browserapi.Handler) (Window, error)

	// Floating creates a new floating window at coordinates,
	// with static width and height.
	Floating(h Floating, cfg component.FloatingConfig) (Window, error)

	// Bar creates a status bar with Orientation and Handler.
	// Bars differ from Split and Floating windows in that they can't
	// be in focus and can only receive mouse events.
	Bar(browserapi.BarConfig, tui.Handler) error

	// Window returns a window with the given window ID or returns false
	// if now window with that ID exists.
	Window(uint64) (Window, bool)
}

// TabManager is the interface that groups tab management methods.
type TabManager interface {
	// Tab creates a new tab with h and returns a handle that can be
	// used with the rest of methods that take a browser.Handler.
	// URI is used to uniquely identify a tab and name is used as a label
	// to display it in the tab bar.
	Tab(uri workspaceapi.URI, icon rune, name string, h browserapi.Handler) (
		browserapi.Handler, error,
	)

	// SetTabName sets the title and attributes of the title of the given tab.
	// If the given browserapi.Handler is not a tab, then this method returns an error.
	SetTabName(workspaceapi.URI, string, term.Attributes) error
}

// Notifications is the interface that wraps methods to display
// messages to the user.
type Notifications interface {
	Notify(level notifications.Level, msg string, args ...interface{}) error
	NotifyOnce(level notifications.Level, msg string, args ...interface{}) error
}

// ResourceOpener is the interface that wraps the method Open.
type ResourceOpener interface {
	Open(resource workspaceapi.URI) (browserapi.Handler, error)
	Resource(workspaceapi.URI) (browserapi.Handler, bool)
}

// EventPublisher is the interface that wraps the method PublishEvent.
type EventPublisher interface {
	PublishEvent(term.Event) error
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

// FuncHandler returns a Handler by wrapping a tui.Handler
// with an Close callback.
func FuncHandler(h tui.Handler, doClose func() error) browserapi.Handler {
	return &closeHandler{Handler: h, doClose: doClose}
}

// NopHandler returns a Handler by wrapping a tui.Handler
// with an nop Close callback.
func NopHandler(h tui.Handler) browserapi.Handler {
	return &closeHandler{Handler: h, doClose: func() error { return nil }}
}

// StaticFloating wraps a Handler and returns a Floating that always
// return the same Dimensions values.
func StaticFloating(h browserapi.Handler, width, height int) Floating {
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
func FuncFloating(h browserapi.Handler, dimFn func() (int, int)) Floating {
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
	browserapi.Handler
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

type staticFloating struct {
	browserapi.Handler
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
