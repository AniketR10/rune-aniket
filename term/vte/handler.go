package vte

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	schemeapi "unstable.build/go-tui/api/scheme"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

var _ tui.Handler = (*Handler)(nil)

// Handler is a terminal emulator that satisfies tui.Handler.
type Handler struct {
	comp          *Component
	publisher     browser.EventPublisher
	notifications browser.Notifications

	mouse       *text.Mouse
	mouseDriver *mouseDriver

	closed        bool
	width, height int
	updateCh      chan struct{}
	sema          chan struct{}
}

// NewHandler allocates storage for a new Handler and initializes it.
func NewHandler(
	publisher browser.EventPublisher, n browser.Notifications,
	terminal schemeapi.Terminal, executor schemeapi.Executor,
	config Config, initialCmd string,
) (*Handler, error) {
	ret := new(Handler)
	err := ret.Init(publisher, n, terminal, executor, config, initialCmd)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// Init initializes this handler.
func (e *Handler) Init(
	publisher browser.EventPublisher, n browser.Notifications,
	termapi schemeapi.Terminal, executor schemeapi.Executor,
	config Config, initialCmd string,
) error {
	e.publisher = publisher
	e.notifications = n

	comp, err := NewComponent(termapi, executor, config)
	if err != nil {
		return err
	}
	e.comp = comp

	// set size hint before running firsrt program so output is correctly captured
	if config.WidthHint != 0 || config.HeightHint != 0 {
		err := e.comp.Resize(config.WidthHint, config.HeightHint)
		if err != nil {
			return fmt.Errorf("set initial pty size: %v", err)
		}
	}
	if initialCmd != "" {
		if err := e.comp.WriteToPty([]byte(initialCmd)); err != nil {
			return fmt.Errorf("write to pty: %v", err)
		}
	}
	e.mouseDriver = &mouseDriver{t: e.comp, clipboard: config.Clipboard}
	e.mouse = text.NewMouse(e.mouseDriver)

	e.updateCh = make(chan struct{}, 1)
	e.sema = make(chan struct{})
	go func() {
		logErr := e.comp.Run(e.updateCh)
		_ = e.publisher.PublishEvent(term.Event{Type: term.EventNone})
		if logErr != nil && !errors.Is(logErr, io.EOF) && !errors.Is(logErr, context.Canceled) {
			e.log(log.ErrorLevel, "terminal run: %v", logErr)
			e.notifications.Notify(notifications.LevelError, "terminal run: %v", logErr)
		} else {
			e.log(log.DebugLevel, "terminal run: ok")
		}
	}()

	go func() {
		for {
			select {
			case <-e.sema:
				e.sema <- struct{}{}
			case _, ok := <-e.updateCh:
				if !ok {
					return
				}
			}
			err = e.publisher.PublishEvent(term.Event{Type: term.EventInterrupt})
			if err != nil {
				e.log(log.ErrorLevel, "interrupt: %s", err)
			}
		}
	}()

	return nil
}

// Component returns the underlying vte.Component.
func (e *Handler) Component() *Component {
	return e.comp
}

// Resize satisfies tui.Component.
func (e *Handler) Resize(width, height int) {
	// avoid divisions by 0 in terminal impl
	if width == 0 || height == 0 {
		return
	}
	e.width, e.height = width, height

	err := e.comp.Resize(width, height)
	if err != nil {
		e.log(log.ErrorLevel, "terminal set size: %s", err)
		// do not notify if already closed
		if !e.closed {
			e.notifications.Notify(notifications.LevelError, "terminal set size: %v", err)
		}
	}
}

// Draw satisfies tui.Component.
func (e *Handler) Draw(w term.Writer) {
	e.comp.Draw(w)
}

// Handle satisfies tui.Handler.
func (e *Handler) Handle(ev term.Event) (exit, handled bool) {
	exit, handled, raw := e.handleInput(ev)
	if exit || handled || len(raw) == 0 {
		return
	}

	select {
	case e.sema <- struct{}{}:
	case _, ok := <-e.updateCh:
		if !ok {
			exit = true
		}
		e.log(log.TraceLevel, "handle: closed update chan")
		return
	}
	defer func() { <-e.sema }()

	err := e.comp.WriteToPty(raw)
	if err != nil {
		e.log(log.ErrorLevel, "write to pty: %s", err)
		e.notifications.Notify(notifications.LevelError, "write to pty: %v", err)
		return
	}

	timer := time.NewTimer(50 * time.Millisecond)
	defer timer.Stop()

	select {
	case <-timer.C:
	case _, ok := <-e.updateCh:
		if !ok {
			exit = true
		}
		handled = true
	}
	// TODO set title via browser.Component if it has changed
	// if e.terminal.Title() != e.currTitle {
	// 	e.browser.SetTitle()
	// }
	return
}

// OnFocusChange allows clients to report whether this vte.Handler is on focus or not.
func (e *Handler) OnFocusChange(inFocus bool) {
	reportFocusMode := e.comp.IsReportFocusMode()
	e.log(log.TraceLevel, "OnFocusChange(inFocus=%t, reportFocusMode=%t)",
		inFocus, reportFocusMode)
	if !reportFocusMode {
		return
	}
	var cmd string
	if inFocus {
		cmd = "I"
	} else {
		cmd = "O"
	}
	err := e.comp.WriteToPty([]byte(fmt.Sprintf("\x1b[%s", cmd)))
	if err != nil {
		e.notifications.Notify(notifications.LevelError, "failed to report focus change to pty: %v", err)
	}
}

// Cursor satisfies tui.Handler.
func (e *Handler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	if !e.comp.CursorVisible() {
		return term.Coordinates{}, 0, false
	}

	return e.comp.CursorCoordinates(), e.comp.CursorStyle(), true
}

// Man satisfies tui.Handler.
func (e *Handler) Man() tui.Manual {
	panic("TODO")
}

// Close closes this terminal emulator and all the resources
// associated with it.
func (e *Handler) Close() error {
	e.log(log.TraceLevel, "close called")

	if e.closed {
		return nil
	}

	// undo circular dependency
	e.mouseDriver.clipboard = nil
	e.mouseDriver = nil
	e.closed = true

	var ret error
	if err := e.comp.Close(); err != nil {
		ret = multierr.Append(ret, err)
		e.log(log.ErrorLevel, "terminal close: %s", err)
	}
	// we can't remove /dev/pts files so leave it up to the system
	return ret
}

func (e *Handler) handleInput(ev term.Event) (exit, handled bool, raw []byte) {
	exit = e.closed
	if exit {
		e.log(log.TraceLevel, "input: exit")
		return
	}

	if ev.Type == term.EventMouse {
		e.mouseDriver.hookRawBytes = nil
		exit, handled = e.mouse.Handle(ev)
		raw = e.mouseDriver.hookRawBytes
		// raw bytes should be sent directly only
		// if we didn't handle mouse event
		if len(raw) != 0 {
			handled = false
		}
		e.log(log.TraceLevel, "input: mouse: exit=%t, handled=%t, raw=%q",
			exit, handled, raw)
		return
	}

	switch ev.Key {
	case term.KeyCtrlL:
		e.mouseDriver.ClearSelection()
	}

	// we cannot simply send raw bytes coming from termbox.
	// The running program sends escape sequences to e.terminal via stdout which
	// configure the program's I/O mode and so this might not might not necessarily
	// match termbox's configuration.
	raw, ok := mapKeyToEscapeSequence(e.comp, ev)
	if !ok {
		raw = ev.Raw
	}
	return
}

func (e *Handler) log(level log.Level, msg string, args ...any) {
	log.WithFields(log.Fields{
		logging.KeyClass: "emulator.Handler",
	}).Logf(level, msg, args...)
}
