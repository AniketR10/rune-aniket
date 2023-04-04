package test

import (
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/browser"
)

// WindowToAPIWindow is a convenience method to work
// with browserapi.Window and browser.Window in the same package.
type WindowToAPIWindow struct {
	Win browser.Window
}

func (a WindowToAPIWindow) SetContent(h browserapi.Handler) error {
	return a.Win.SetContent(h)
}

func (a WindowToAPIWindow) Focus() (bool, error) {
	return a.Win.Focus()
}

func (a WindowToAPIWindow) Close() error {
	return a.Win.Close()
}

func (a WindowToAPIWindow) Content() (browserapi.Handler, error) {
	return a.Win.Content()
}

func (a WindowToAPIWindow) ID() uint64 {
	return a.Win.ID()
}

// WindowFromAPIWindow is a convenience method to work
// with browserapi.Window and browser.Window in the same package.
type WindowFromAPIWindow struct {
	Win browserapi.Window
}

func (a WindowFromAPIWindow) SetContent(h browserapi.Handler) error {
	return a.Win.SetContent(h)
}

func (a WindowFromAPIWindow) Content() (browserapi.Handler, error) {
	h, err := a.Win.(interface {
		Content() (browserapi.Browser, error)
	}).Content()
	return h.(browserapi.Handler), err
}

func (a WindowFromAPIWindow) ID() uint64 {
	return a.Win.(interface{ ID() uint64 }).ID()
}

func (a WindowFromAPIWindow) Focus() (bool, error) {
	return a.Win.Focus()
}

func (a WindowFromAPIWindow) Close() error {
	return a.Win.Close()
}

func (w WindowFromAPIWindow) Closed() bool {
	return w.Win.(interface{ Closed() bool }).Closed()
}
