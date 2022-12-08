package test

import (
	"context"

	"unstable.build/go-tui/browser"
)

type noopWindow struct{}

func (w noopWindow) Content() (browser.Handler, error)       { return nil, nil }
func (w noopWindow) SetContent(h browser.Handler) error      { return nil }
func (w noopWindow) Close(context.Context) error             { return nil }
func (w noopWindow) OnWindowClosed(fn func(context.Context)) {}
func (w noopWindow) ID() uint64                              { return 0 }
func (w noopWindow) Focus() (bool, error)                    { return false, nil }
func (w noopWindow) Closed() bool                            { return false }

// NopWindow returns a window that does nothing.
func NopWindow() browser.Window {
	return noopWindow{}
}
