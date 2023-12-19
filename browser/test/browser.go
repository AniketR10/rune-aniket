package test

import (
	"fmt"

	"unstable.build/go-tui"
	browserapi "unstable.build/go-tui/api/browser"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component"
	notifications "unstable.build/go-tui/component/notifications"
	term "unstable.build/go-tui/term"
)

// BrowserFromAPIBrowser wraps a browserapi.Browser and returns
// a browser.Browser.
func BrowserFromAPIBrowser(b browserapi.Browser) browser.Browser {
	return toBrowser{b: b}
}

type toBrowser struct {
	b browserapi.Browser
}

func (b toBrowser) Focus() (browser.Window, error) {
	win, err := b.b.Focus()
	if err != nil {
		return nil, err
	}
	return WindowFromAPIWindow{Win: win}, nil
}

func (b toBrowser) SetFocus(win browser.Window) (browser.Window, error) {
	var inWin browserapi.Window
	if a, ok := win.(WindowFromAPIWindow); ok {
		inWin = a.Win
	} else {
		inWin = WindowToAPIWindow{Win: win}
	}
	retWin, err := b.b.SetFocus(inWin)
	if err != nil {
		return nil, err
	}
	return WindowFromAPIWindow{Win: retWin}, nil
}

func (b toBrowser) Split(
	o browserapi.Orientation, win browser.Window, h browserapi.Handler,
) (browser.Window, error) {
	var inWin browserapi.Window
	if a, ok := win.(WindowFromAPIWindow); ok {
		inWin = a.Win
	} else {
		inWin = WindowToAPIWindow{Win: win}
	}
	retWin, err := b.b.Split(o, inWin, h)
	if err != nil {
		return nil, err
	}
	return WindowFromAPIWindow{Win: retWin}, nil
}

func (b toBrowser) Floating(
	h browser.Floating, cfg component.FloatingConfig,
) (browser.Window, error) {
	retWin, err := b.b.Floating(h, cfg)
	if err != nil {
		return nil, err
	}
	return WindowFromAPIWindow{Win: retWin}, nil
}

func (b toBrowser) Bar(o browserapi.Orientation, h tui.Handler) error {
	return b.b.Bar(o, h)
}

func (b toBrowser) Tab(uri workspaceapi.URI, name string, h browserapi.Handler) (browserapi.Handler, error) {
	return b.b.Tab(uri, name, h)
}

func (b toBrowser) Window(id uint64) (browser.Window, bool) {
	return NopWindow(), true
}

func (b toBrowser) Notify(level notifications.Level, msg string, args ...interface{}) error {
	return b.b.Notify(level, msg, args...)
}

func (b toBrowser) Open(resource workspaceapi.URI) (browserapi.Handler, error) {
	return b.b.Open(resource)
}

func (b toBrowser) Resource(u workspaceapi.URI) (browserapi.Handler, bool) {
	return NewTestHandler(), true
}

func (b toBrowser) PublishEvent(ev term.Event) error {
	if ev.Type == term.EventInterrupt {
		return b.b.Interrupt()
	} else if ev.Type == term.EventNone {
		return b.b.PublishEventNone()
	}
	return fmt.Errorf("cannot publish event type: %v", ev.Type)
}

func (b toBrowser) Close() error {
	return b.b.Close()
}
