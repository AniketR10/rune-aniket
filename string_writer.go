package fractal

import (
	"bytes"

	"termbox"
)

type StringWriter struct {
	cellbuf       []Cell
	buffer        bytes.Buffer
	width, height int
	CursorCh      rune
}

func NewStringWriter(width, height int) (t *StringWriter) {
	t = new(StringWriter)
	t.Resize(width, height)
	t.CursorCh = '▐'
	return
}

func (w *StringWriter) Resize(width, height int) {
	w.width, w.height = width, height
	w.cellbuf = make([]Cell, width*height)
}

func (w *StringWriter) Write(x, y int, ch rune, fg, bg termbox.Attribute) error {
	if x >= w.width || y >= w.height {
		return nil
	}
	idx := y*w.width + x
	w.cellbuf[idx] = Cell{Coordinates: Coordinates{X: x, Y: y}, Ch: ch, Fg: fg, Bg: bg}
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
		switch ch {
		case '\n':
			fallthrough
		case 0:
			fallthrough
		case '\t':
			ch = ' '
		}
		if _, err = w.buffer.WriteRune(ch); err != nil {
			return
		}
	}
	return
}

func (w *StringWriter) Cells() []Cell {
	return w.cellbuf
}

func (w *StringWriter) Clear(_, _ termbox.Attribute) (err error) {
	w.cellbuf = make([]Cell, w.width*w.height)
	w.buffer.Reset()
	return
}

func (w *StringWriter) String() string {
	return w.buffer.String()
}

func (w *StringWriter) SetCursor(pos Coordinates) {
	i := pos.X + pos.Y*w.width
	if i < len(w.cellbuf) {
		w.cellbuf[i].Ch = w.CursorCh
	}
}
