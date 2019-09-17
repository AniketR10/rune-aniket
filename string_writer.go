package fractal

import (
	"bytes"

	"termbox"
)

type stringWriter struct {
	cellbuf       []termbox.Cell
	buffer        bytes.Buffer
	width, height int
	CursorCh      rune
}

func newStringWriter(width, height int) (t *stringWriter) {
	t = new(stringWriter)
	t.Resize(width, height)
	t.CursorCh = '▐'
	return
}

func (w *stringWriter) Resize(width, height int) {
	w.width, w.height = width, height
	w.cellbuf = make([]termbox.Cell, width*height)
}

func (w *stringWriter) Write(x, y int, ch rune, fg, bg termbox.Attribute) error {
	if x >= w.width || y >= w.height || x < 0 || y < 0 {
		return nil
	}
	idx := y*w.width + x
	w.cellbuf[idx] = termbox.Cell{Ch: ch, Fg: fg, Bg: bg}
	return nil
}

func (w *stringWriter) Flush() (err error) {
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

func (w *stringWriter) Cells() []termbox.Cell {
	return w.cellbuf
}

func (w *stringWriter) Clear(_, _ termbox.Attribute) (err error) {
	w.cellbuf = make([]termbox.Cell, w.width*w.height)
	w.buffer.Reset()
	return
}

func (w *stringWriter) String() string {
	return w.buffer.String()
}

func (w *stringWriter) SetCursor(pos Coordinates) {
	i := pos.X + pos.Y*w.width
	if i < len(w.cellbuf) {
		w.cellbuf[i].Ch = w.CursorCh
	}
}
