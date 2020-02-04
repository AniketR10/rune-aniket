package editor

import (
	"github.com/ernestrc/fractal/cell"
	"github.com/ernestrc/fractal/term"
)

type delClip struct {
	writer    cell.Writer
	clipboard Clipboard
}

// WithCopyDelete installs a cell.Writer to a Buffer which persists all the deleted
// content to a Clipboard.
func WithCopyDelete(clipboard Clipboard, buf *cell.Buffer) {
	c := new(delClip)
	c.clipboard = clipboard

	writer := buf.Writer()
	c.writer = writer
	buf.WithWriter(c)
}

func (c *delClip) Insert(at term.Coordinates, str string) (
	from, to term.Coordinates,
) {
	return c.writer.Insert(at, str)
}

func (c *delClip) Delete(from, to term.Coordinates) (
	start, end term.Coordinates, str string,
) {
	start, end, str = c.writer.Delete(from, to)
	c.clipboard.Set(str)
	return
}
