package editor

import (
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

type delClip struct {
	clipboard Clipboard
	pub       cell.Publisher
}

// WithCopyDelete installs a cell.Writer to a Buffer which persists all the deleted
// content to a Clipboard.
func WithCopyDelete(clipboard Clipboard, buf *cell.Buffer) {
	c := new(delClip)
	c.clipboard = clipboard
	c.pub = buf
	buf.Subscribe(c)
}

func (c *delClip) OnWillInsert(at term.Coordinates, str string) {
}

func (c *delClip) OnDidInsert(from, to term.Coordinates) {
}

func (c *delClip) OnWillDelete(from, to term.Coordinates) {
}

func (c *delClip) OnDidDelete(start, end term.Coordinates, str string) {
	c.clipboard.Set(Paste{Data: str})
	return
}

func (c *delClip) Unsubscribe() {
	if c.pub == nil {
		return
	}
	c.pub.Unsubscribe(c)
	c.pub = nil
}
