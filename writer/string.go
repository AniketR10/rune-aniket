package writer

import (
	"bytes"

	"github.com/ernestrc/fractal"
)

type StringWriter struct {
	cellbuf       []fractal.Cell
	buffer        bytes.Buffer
	width, height int
}

func String(width, height int) (t *StringWriter) {
	t = new(StringWriter)
	t.width, t.height = width, height
	t.cellbuf = make([]fractal.Cell, width*height)
	return
}

func (w *StringWriter) Write(x, y int, ch rune, fg, bg fractal.Attribute) error {
	if x >= w.width || y >= w.height {
		return nil
	}
	idx := y*w.width + x
	w.cellbuf[idx] = fractal.Cell{Coordinates: fractal.Coordinates{X: x, Y: y}, Ch: ch, Fg: fg, Bg: bg}
	return nil
}

func (w *StringWriter) Flush() (err error) {
	for i, c := range w.cellbuf {
		if i != 0 && i%w.width == 0 {
			if _, err = w.buffer.WriteRune('\n'); err != nil {
				return
			}
		}
		ch := c.Ch
		if ch == 0 {
			ch = ' '
		}
		if _, err = w.buffer.WriteRune(ch); err != nil {
			return
		}
	}
	return
}

func (w *StringWriter) Cells() []fractal.Cell {
	return w.cellbuf
}

func (w *StringWriter) Clear(_, _ fractal.Attribute) (err error) {
	w.cellbuf = make([]fractal.Cell, w.width*w.height)
	w.buffer.Reset()
	return
}

func (w *StringWriter) String() string {
	return w.buffer.String()
}
