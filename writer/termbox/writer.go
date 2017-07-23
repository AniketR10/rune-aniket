package termbox

import (
	"github.com/ernestrc/fractal"
	termbox "github.com/nsf/termbox-go"
)

// TODO deal with height and width
type TermboxWriter struct {
	buffer        []termbox.Cell
	Width, Height int
}

func New(width, height int) (t *TermboxWriter) {
	t = new(TermboxWriter)
	t.buffer = termbox.CellBuffer()
	return
}

func (w *TermboxWriter) Write(x, y int, ch rune) error {
	idx := y*w.Width + x
	c := w.buffer[idx]
	w.buffer[idx] = termbox.Cell{Ch: ch, Fg: c.Fg, Bg: c.Bg}
	return nil
}

func (w *TermboxWriter) SetAttributes(x, y int, fg fractal.Attribute, bg fractal.Attribute) {
	idx := y*w.Width + x
	w.buffer[idx] = termbox.Cell{Ch: w.buffer[idx].Ch, Fg: termbox.Attribute(fg), Bg: termbox.Attribute(bg)}
}

func (w *TermboxWriter) Flush() (err error) {
	err = termbox.Flush()
	w.buffer = termbox.CellBuffer()
	return
}

func (w *TermboxWriter) Clear(fg, bg fractal.Attribute) (err error) {
	err = termbox.Clear(termbox.Attribute(fg), termbox.Attribute(bg))
	w.buffer = termbox.CellBuffer()
	return
}
