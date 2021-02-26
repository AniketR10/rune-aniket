package editor

import (
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

type delClip struct {
	clipboard Clipboard
	pub       cell.Publisher
	cur       *Cursor
	mode      SelectMode
}

// WithCopyDelete installs a cell.Writer to a Buffer which persists all the deleted
// content to a Clipboard.
func WithCopyDelete(clipboard Clipboard, cur *Cursor, buf *cell.Buffer) {
	c := new(delClip)
	c.clipboard = clipboard
	c.pub = buf
	c.cur = cur
	buf.SubscribeUsage(c)
}

func (c *delClip) OnWillInsert(at term.Coordinates, str string) {
}

func (c *delClip) OnDidInsert(from, to term.Coordinates) {
}

func (c *delClip) OnWillDelete(from, to term.Coordinates) {
	var ok bool
	c.mode, ok = c.cur.SelectionMode()
	if !ok {
		c.mode = StandardSelection
	}
}

func (c *delClip) OnDidDelete(start, end term.Coordinates, str string) {
	c.clipboard.Set(Paste{Data: str, Metadata: c.mode})
	return
}
