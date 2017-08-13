package fractal

import (
	"termbox"
)

type TermboxWriter struct {
}

func Termbox() *TermboxWriter {
	return new(TermboxWriter)
}

func (w *TermboxWriter) Write(x, y int, ch rune, fg, bg termbox.Attribute) error {
	termbox.SetCell(x, y, ch, termbox.Attribute(fg), termbox.Attribute(bg))
	return nil
}

func (w *TermboxWriter) Flush() error {
	return termbox.Flush()
}

func (w *TermboxWriter) Clear(fg, bg termbox.Attribute) (err error) {
	err = termbox.Clear(termbox.Attribute(fg), termbox.Attribute(bg))
	return
}
