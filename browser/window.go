package browser

import (
	"errors"

	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/handler"
)

type browserWindow struct {
	parent *Component
	win    handler.Window
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

func (w *browserWindow) Content() (browserapi.Handler, error) {
	h := w.win.Content().(browserapi.Handler)
	t, ok := h.(*Tab)
	if !ok {
		return h.(*browserContent).Handler, nil
	}
	return t, nil
}

func (w *browserWindow) SetContent(h browserapi.Handler) error {
	if w.parent == nil {
		return errors.New("window is closing")
	}
	return w.parent.tryUpdateWindowContent(w, h)
}

func (w *browserWindow) Close() error {
	if w.parent == nil {
		return nil
	}

	parent := w.parent

	err := parent.closeWindow(w)
	if err != nil {
		parent.setError(err)
		return err
	}

	w.parent = nil

	return nil
}
