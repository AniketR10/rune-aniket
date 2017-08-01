package writer

import (
	"bytes"

	"github.com/ernestrc/fractal"
	termbox "github.com/nsf/termbox-go"
)

type StringWriter struct {
	cellbuf       []fractal.Cell
	buffer        bytes.Buffer
	width, height int
}

func String(width, height int) (t *StringWriter) {
	t = new(StringWriter)
	t.Resize(width, height)
	return
}

func (w *StringWriter) Resize(width, height int) {
	w.width, w.height = width, height
	w.cellbuf = make([]fractal.Cell, width*height)
}

func (w *StringWriter) Write(x, y int, ch rune, fg, bg termbox.Attribute) error {
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

func (w *StringWriter) Clear(_, _ termbox.Attribute) (err error) {
	w.cellbuf = make([]fractal.Cell, w.width*w.height)
	w.buffer.Reset()
	return
}

func (w *StringWriter) String() string {
	return w.buffer.String()
}
