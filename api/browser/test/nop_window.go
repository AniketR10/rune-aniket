package test

import api "unstable.build/go-tui/api/browser"

type noopWindow struct{}

func (w noopWindow) Content() (api.Handler, error)  { return nil, nil }
func (w noopWindow) SetContent(h api.Handler) error { return nil }
func (w noopWindow) Close() error                   { return nil }
func (w noopWindow) OnWindowClosed(fn func())       {}
func (w noopWindow) ID() uint64                     { return 0 }
func (w noopWindow) Focus() (bool, error)           { return false, nil }

// NopWindow returns a window that does nothing.
func NopWindow() api.Window {
	return noopWindow{}
}
