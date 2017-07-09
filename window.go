package less

import (
	"bytes"
	"io"

	termbox "github.com/nsf/termbox-go"
)

type Window struct {
	buffer      *Buffer // content buffer
	config      *Config // user configuration
	xTermOffset int     // x offset from termbox window
	yTermOffset int     // y offset from termbox window
	xmaxoffset  int     // max x content offset
	ymaxoffset  int     // max y content offset
	xoffset     int     // current x content offset
	yoffset     int     // current y content offset
	height      int     // window height
	width       int     // window width
}

// NormalizeOffsets should be called before drawing and after manually modifying X/Y offsets
func (h *Buffer) NormalizeOffsets() {
	if h.Xoffset < 0 {
		h.Xoffset = 0
	} else if h.Xoffset > h.xmaxoffset {
		h.Xoffset = h.xmaxoffset
	}
	if h.Yoffset < 0 {
		h.Yoffset = 0
	} else if h.Yoffset > h.ymaxoffset {
		h.Yoffset = h.ymaxoffset
	}
}

func (w *Window) moveUp() {

}

func (w *Window) moveDown() {

}

func (w *Window) moveLeft() {

}

func (w *Window) moveRight() {

}

func (w *Window) SetBuffer(buffer *Buffer) error {
	var err error
	var c rune
	var currX int
	h.columns, h.rows = 0, 0
	view := bytes.NewBuffer(h.contentBuf.Bytes())
	for i := 0; ; i++ {
		if c, _, err = view.ReadRune(); err != nil {
			break
		}

		switch c {
		case '\n':
			h.rows++
			if currX > h.columns {
				h.columns = currX
			}
			currX = 0
		case '\t':
			currX += h.config.Tabspaces
		default:
			currX++
		}

		// collect x, y coordinates
		prev := h.cells[i]
		h.cells[i] = cell{fg: prev.fg, bg: prev.bg, x: currX, y: h.rows, i: i}
	}

	if h.rows <= h.weight {
		h.ymaxoffset = 0
	} else {
		// FIXME + h.config.CmdBarHeight
		// h.ymaxoffset = h.rows - h.height + h.config.CmdBarHeight
		h.ymaxoffset = h.rows - h.weight
	}

	if h.config.Wrap {
		h.xmaxoffset = 0
	} else if h.columns >= h.width {
		h.xmaxoffset = h.columns - h.width
	} else {
		h.xmaxoffset = 0
	}

	if err != io.EOF {
		return err
	}

	return nil
}

// x/yoffset is the offset from the content
// x/ystart is the offset in the cell grid
func (w *Window) draw(data *bytes.Buffer, palette map[int]cell,
	xoffset, yoffset, xstart, ystart int) (x, y int, err error) {
	var r rune
	x = xstart
	y = ystart

	currxoffset := xoffset

	// draw until we've filled all available cells
	for i := 0; y-ystart < w.weight; i++ {
		if r, _, err = data.ReadRune(); err != nil {
			break
		}

		// wrap or skip content
		if x-xstart == w.Width {
			if w.config.Wrap {
				y++
				x = xstart
			} else {
				if r == '\n' {
					y++
					x = xstart
					currxoffset = xoffset
				}
				continue
			}
		}

		switch r {
		case '\n':
			if yoffset > 0 {
				yoffset--
			} else {
				y++
				x = xstart
				currxoffset = xoffset
			}
		case '\t':
			if yoffset != 0 {
				continue
			}
			if currxoffset <= 0 {
				x += w.config.Tabspaces
			} else {
				currxoffset -= w.config.Tabspaces
			}
		default:
			if yoffset != 0 {
				continue
			}
			if currxoffset > 0 {
				currxoffset--
				continue
			}
			if palette != nil {
				c := palette[i]
				termbox.SetCell(x, y, r, c.fg, c.bg)
			} else {
				termbox.SetCell(x, y, r, w.config.Fg, w.config.Bg)
			}
			x++
		}
	}

	return x, y, err
}
