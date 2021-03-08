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
	parent      *Component
	id          string
	closer      io.Closer
	handler     tui.Handler
	free        bool
	subscribers []TabSubscriber
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
	for _, sub := range b.subscribers {
		sub.OnFocus(b)
	}
}

func (b *Tab) setFree() {
	b.free = true
	for _, sub := range b.subscribers {
		sub.OnFree(b)
	}
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

// ID returns the identifier of this tab.
func (b *Tab) ID() string {
	return b.id
}

// Handler returns the tui.Handler responsible for drawing
// the contents of this tab.
func (b *Tab) Handler() tui.Handler {
	return b.handler
}

// Closer returns the closer passed to browser.Component.NewTab,
// which is used when tab is closed via
func (b *Tab) Closer() io.Closer {
	return b.closer
}

// TabSubscriber is a subscriber of tab focus or free operations.
type TabSubscriber interface {
	OnFocus(*Tab)
	OnFree(*Tab)
}

// Subscribe subscribes sub to OnFocus and OnFree operations.
func (b *Tab) Subscribe(sub TabSubscriber) {
	b.subscribers = append(b.subscribers, sub)
	if b.free {
		sub.OnFree(b)
	} else {
		sub.OnFocus(b)
	}
}
