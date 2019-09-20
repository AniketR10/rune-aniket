package fractal

import (
	"termbox"
)

// used for benchmarks
type noopWriter struct{}

func (w noopWriter) Write(x, y int, ch rune, fg, bg termbox.Attribute) error {
	return nil
}

func (w noopWriter) Flush() (err error) {
	return
}

func (w noopWriter) Clear(_, _ termbox.Attribute) (err error) {
	return
}

func (w noopWriter) SetCursor(pos Coordinates) {
}
