package test

import (
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/browser"
)

type noopWindow struct {
	content browserapi.Handler
}

func (w *noopWindow) Content() (browserapi.Handler, error)  { return w.content, nil }
func (w *noopWindow) SetContent(h browserapi.Handler) error { w.content = h; return nil }
func (w *noopWindow) Close() error {
	if w.content == nil {
		return nil
	}
	err := w.content.Close()
	w.content = nil
	return err
}
func (w *noopWindow) ID() uint64           { return 0 }
func (w *noopWindow) Focus() (bool, error) { return false, nil }
func (w *noopWindow) Closed() bool         { return false }
func (w *noopWindow) IsFloating() bool     { return false }

// NopWindow returns a window that does nothing.
func NopWindow() browser.Window {
	return &noopWindow{}
}
