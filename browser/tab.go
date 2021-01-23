package browser

import (
	"io"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

// Tab is a structure that represents a tab in a Browser.Component.
// It satisfies browser.Handler interface so it can be used
// with browser.Browser API. See browser.Component.NewTab for more details.
type Tab struct {
	parent  *Component
	id      string
	closer  io.Closer
	handler tui.Handler
	free    bool
}

// newTab allocates storage for a new tab and initializes it.
func newTab(c *Component, id string, h tui.Handler, f io.Closer) *Tab {
	ret := new(Tab)
	ret.init(c, id, h, f)
	return ret
}

// Init initializes this tab with id, h as the Handler, and f as the
// io.Closer handle.
func (b *Tab) init(c *Component, id string, h tui.Handler, f io.Closer) {
	b.parent = c
	b.id = id
	b.closer = f
	b.handler = h
	b.free = true
}

func (b *Tab) setWindow() {
	b.free = false
}

func (b *Tab) setFree() {
	b.free = true
}

// Resize satisfies tui.Component
func (b *Tab) Resize(width, height int) {
	b.handler.Resize(width, height)
}

// Draw satisfies tui.Component
func (b *Tab) Draw(w term.Writer) {
	b.handler.Draw(w)
}

// Handle satisfies tui.Handler
func (b *Tab) Handle(ev term.Event) (exit, handled bool) {
	return b.handler.Handle(ev)
}

// Cursor satisfies tui.Handler
func (b *Tab) Cursor() (pos term.Coordinates, show bool) {
	return b.handler.Cursor()
}

// Man satisfies tui.Handler
func (b *Tab) Man() tui.Manual {
	return b.handler.Man()
}

func (b *Tab) doClose() error {
	if b.closer != nil {
		err := b.closer.Close()
		b.closer = nil
		return err
	}
	return nil
}

// OnUnmount satisfies browser.Handler.
func (b *Tab) OnUnmount() error {
	b.setFree()
	return nil
}

// Closer returns the closer passed to browser.Component.NewTab,
// which is used when tab is closed via
func (b *Tab) Closer() io.Closer {
	return b.closer
}
