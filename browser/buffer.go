package browser

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

// FlusherCloser used to abstract editor.FileBufer
type FlusherCloser interface {
	Flush() error
	Close() error
}

// buffer is a wrap structure to be able to add certain
// properties to a tui.Handler.
type buffer struct {
	parent        *Component
	name          string
	flusherCloser FlusherCloser
	handler       Handler
	win           *browserWindow
	free          bool
}

// newBuffer allocates storage for a new buffer and initializes it.
func newBuffer(c *Component, name string, h tui.Handler, f FlusherCloser) *buffer {
	ret := new(buffer)
	ret.init(c, name, h, f)
	return ret
}

// Init initializes this buffer with name, h as the Handler, and f as the
// FlusherCloser handle.
func (b *buffer) init(c *Component, name string, h tui.Handler, f FlusherCloser) {
	b.parent = c
	b.name = name
	b.flusherCloser = f
	b.handler = NopHandler(h)
	b.setFree()
}

func (b *buffer) setWindow(win *browserWindow) {
	b.free = false
	b.win = win
}

func (b *buffer) setFree() {
	b.free = true
	b.win = nil
}

// Resize satisfies tui.Component
func (b *buffer) Resize(width, height int) {
	b.handler.Resize(width, height)
}

// Draw satisfies tui.Component
func (b *buffer) Draw(w term.Writer) {
	b.handler.Draw(w)
}

// Handle satisfies tui.Handler
func (b *buffer) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = b.handler.Handle(ev)
	if exit {
		b.parent.RemoveWindowBuffer(b.win)
	}
	return
}

// Cursor satisfies tui.Handler
func (b *buffer) Cursor() (pos term.Coordinates, show bool) {
	return b.handler.Cursor()
}

// Man satisfies tui.Handler
func (b *buffer) Man() tui.Manual {
	return b.handler.Man()
}

// Close satisfies io.Closer
func (b *buffer) Close() error {
	if b.flusherCloser != nil {
		err := b.flusherCloser.Close()
		b.flusherCloser = nil
		return err
	}
	return nil
}

func (b *buffer) OnUnmount() error {
	b.setFree()
	return nil
}
