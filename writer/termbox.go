package writer

import (
	"github.com/ernestrc/fractal"
	termbox "github.com/nsf/termbox-go"
)

type TermboxWriter struct {
}

func Termbox() *TermboxWriter {
	return new(TermboxWriter)
}

func (w *TermboxWriter) Write(x, y int, ch rune, fg, bg fractal.Attribute) error {
	termbox.SetCell(x, y, ch, termbox.Attribute(fg), termbox.Attribute(bg))
	return nil
}

func (w *TermboxWriter) Flush() (err error) {
	err = termbox.Flush()
	return
}

func (w *TermboxWriter) Clear(fg, bg fractal.Attribute) (err error) {
	err = termbox.Clear(termbox.Attribute(fg), termbox.Attribute(bg))
	return
}
