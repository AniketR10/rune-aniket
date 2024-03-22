package text

import (
	"context"

	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text/clipboard"
)

type delClip struct {
	clipboard  clipboard.Register
	registerID string
	pub        cell.Publisher
	cur        *Cursor
	mode       SelectMode
}

// WithCopyDelete installs a cell.Subscriber to a cell.Buffer
// which persists all the deleted content to a Clipboard.
func WithCopyDelete(
	registerID string, clipboard clipboard.Register,
	cur *Cursor, buf *cell.Buffer,
) {
	c := new(delClip)
	c.clipboard = clipboard
	c.pub = buf
	c.cur = cur
	c.registerID = registerID
	buf.SubscribeUsage(c)
}

func (c *delClip) OnWillEdit(
	ctx context.Context, start, end term.Coordinates, str string,
) {
	if start == end {
		return
	}

	var ok bool
	c.mode, ok = c.cur.SelectionMode()
	if !ok {
		c.mode = StandardSelection
	}
}

func (c *delClip) OnDidEdit(
	ctx context.Context, from, to term.Coordinates, old string,
) {
	if old != "" {
		c.clipboard.Copy(c.registerID, clipboard.Data{Text: old, Metadata: c.mode})
	}
	return
}
