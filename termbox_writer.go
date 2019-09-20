package fractal

import (
	"github.com/nsf/termbox-go"
)

// Writer abstracts termbox write functionality to decouple components from
// termbox, so they're easier to test.
type Writer interface {
	Write(x, y int, r rune, fg termbox.Attribute, bg termbox.Attribute) error
	Flush() error
	Clear(fg, bg termbox.Attribute) error
	SetCursor(Coordinates)
}

type termboxWriter struct{}

func (w *termboxWriter) Write(x, y int, ch rune, fg, bg termbox.Attribute) error {
	termbox.SetCell(x, y, ch, termbox.Attribute(fg), termbox.Attribute(bg))
	return nil
}

func (w *termboxWriter) Flush() error {
	return termbox.Flush()
}

func (w *termboxWriter) Clear(fg, bg termbox.Attribute) (err error) {
	err = termbox.Clear(termbox.Attribute(fg), termbox.Attribute(bg))
	return
}

func (w *termboxWriter) SetCursor(pos Coordinates) {
	termbox.SetCursor(pos.X, pos.Y)
}
