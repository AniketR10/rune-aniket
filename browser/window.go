package browser

import (
	"context"
	"errors"

	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/handler"
)

var (
	_ Window            = WindowFromAPIWindow{}
	_ browserapi.Window = WindowToAPIWindow{}
)

// WindowToAPIWindow is a convenience method to work
// with browserapi.Window and browser.Window in the same package.
type WindowToAPIWindow struct {
	Win Window
}

// WindowFromAPIWindow is a convenience method to work
// with browserapi.Window and browser.Window in the same package.
type WindowFromAPIWindow struct {
	Win browserapi.Window
}

type browserWindow struct {
	parent  *Component
	win     handler.Window
	doClose []func(context.Context)
}

func (w *browserWindow) Focus() (bool, error) {
	return w.win.Focus(), nil
}

func (w *browserWindow) ID() uint64 {
	return w.win.ID()
}

func (w *browserWindow) Closed() bool {
	return w.parent == nil
}

func (w *browserWindow) OnWindowClosed(fn func(context.Context)) {
	w.doClose = append(w.doClose, fn)
}

func (w *browserWindow) Content() (Handler, error) {
	h := w.win.Content().(Handler)
	t, ok := h.(*Tab)
	if !ok {
		return h.(*browserContent).Handler, nil
	}
	return t, nil
}

func (w *browserWindow) SetContent(h Handler) error {
	if w.parent == nil {
		return errors.New("window is closing")
	}
	return w.parent.tryUpdateWindowContent(w, h)
}

func (w *browserWindow) Close(ctx context.Context) error {
	if w.parent == nil {
		return nil
	}

	parent := w.parent
	doClose := w.doClose

	err := parent.closeWindow(w)
	if err != nil {
		parent.setError(err)
		return err
	}

	w.doClose = nil
	w.parent = nil

	for _, fn := range doClose {
		fn(ctx)
	}

	return nil
}

func (a WindowToAPIWindow) SetContent(h browserapi.Handler) error {
	return a.Win.SetContent(h)
}

func (a WindowToAPIWindow) Focus() (bool, error) {
	return a.Win.Focus()
}

func (a WindowToAPIWindow) Close() error {
	// api window is client side so just pass a background ctx
	return a.Win.Close(context.Background())
}

func (a WindowToAPIWindow) OnWindowClosed(fn func(context.Context)) {
	a.Win.OnWindowClosed(fn)
}

func (a WindowToAPIWindow) Content() (browserapi.Handler, error) {
	return a.Win.Content()
}

func (a WindowToAPIWindow) ID() uint64 {
	return a.Win.ID()
}

func (a WindowFromAPIWindow) SetContent(h Handler) error {
	return a.Win.SetContent(h)
}

func (a WindowFromAPIWindow) Content() (Handler, error) {
	h, err := a.Win.(interface {
		Content() (browserapi.Browser, error)
	}).Content()
	return h.(Handler), err
}

func (a WindowFromAPIWindow) ID() uint64 {
	return a.Win.(interface{ ID() uint64 }).ID()
}

func (a WindowFromAPIWindow) Focus() (bool, error) {
	return a.Win.Focus()
}

func (a WindowFromAPIWindow) Close(ctx context.Context) error {
	return a.Win.Close()
}

func (w WindowFromAPIWindow) Closed() bool {
	return w.Win.(interface{ Closed() bool }).Closed()
}

func (a WindowFromAPIWindow) OnWindowClosed(fn func(context.Context)) {
	onCaller, ok := a.Win.(interface{ OnWindowClosed(func(context.Context)) })
	if ok {
		onCaller.OnWindowClosed(fn)
	}
}
