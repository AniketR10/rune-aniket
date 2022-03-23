package text

import (
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

type delClip struct {
	clipboard  Clipboard
	registerID string
	pub        cell.Publisher
	cur        *Cursor
	mode       SelectMode
}

// WithCopyDelete installs a cell.Subscriber to a cell.Buffer
// which persists all the deleted content to a Clipboard.
func WithCopyDelete(
	registerID string, clipboard Clipboard,
	cur *Cursor, buf *cell.Buffer,
) {
	c := new(delClip)
	c.clipboard = clipboard
	c.pub = buf
	c.cur = cur
	c.registerID = registerID
	buf.SubscribeUsage(c)
}

func (c *delClip) OnWillEdit(start, end term.Coordinates, str string) {
	if start == end {
		return
	}

	var ok bool
	c.mode, ok = c.cur.SelectionMode()
	if !ok {
		c.mode = StandardSelection
	}
}

func (c *delClip) OnDidEdit(from, to term.Coordinates, old string) {
	if old != "" {
		c.clipboard.Copy(c.registerID, ClipboardData{Text: old, Metadata: c.mode})
	}
	return
}
