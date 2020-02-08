package editor

import (
	"github.com/ernestrc/fractal/cell"
	"github.com/ernestrc/fractal/term"
)

type delClip struct {
	clipboard Clipboard
}

// WithCopyDelete installs a cell.Writer to a Buffer which persists all the deleted
// content to a Clipboard.
func WithCopyDelete(clipboard Clipboard, buf *cell.Buffer) {
	c := new(delClip)
	c.clipboard = clipboard
	buf.Subscribe(c)
}

func (c *delClip) OnInsert(from, to term.Coordinates, str string) {
}

func (c *delClip) OnDelete(from, to term.Coordinates, str string) {
	c.clipboard.Set(Paste{Data: str})
	return
}
