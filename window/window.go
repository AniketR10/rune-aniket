package window

import (
	"bytes"
	"io"

	"github.com/ernestrc/fractal/buffer"
	termbox "github.com/nsf/termbox-go"
)

type Window struct {
	buffer     *buffer.Buffer      // content buffer
	cells      map[int]buffer.Cell // color information used for printing to window
	wrap       bool                // wrap text
	tabspaces  int                 // tab width
	xmaxoffset int                 // max x content offset
	ymaxoffset int                 // max y content offset
	xoffset    int                 // current x content offset
	yoffset    int                 // current y content offset
	xstart     int                 // x offset from root window
	ystart     int                 // y offset from root window
	height     int                 // window height
	width      int                 // window width
}

func New(initial *buffer.Buffer, tabspaces int, wrap bool) *Window {
	w := new(Window)
	w.cells = make(map[int]buffer.Cell)

	w.buffer = initial
	w.tabspaces = tabspaces
	w.wrap = wrap

	return w
}

func (w *Window) Cells() map[int]buffer.Cell {
	return w.cells
}

func (w *Window) YOffset() int {
	return w.yoffset
}

func (w *Window) XOffset() int {
	return w.xoffset
}

func (w *Window) CanMoveUp() bool {
	return w.yoffset > 0
}

func (w *Window) CanMoveDown() bool {
	return w.yoffset < w.ymaxoffset
}

func (w *Window) CanMoveLeft() bool {
	return w.xoffset > 0
}

func (w *Window) CanMoveRight() bool {
	return w.xoffset < w.xmaxoffset
}

func (w *Window) MoveUp() {
	if w.CanMoveUp() {
		w.yoffset--
	}
}

func (w *Window) MoveDown() {
	if w.CanMoveDown() {
		w.yoffset += 1
	}
}

func (w *Window) MoveLeft() {
	if w.CanMoveLeft() {
		w.xoffset--
	}
}

func (w *Window) MoveRight() {
	if w.CanMoveRight() {
		w.xoffset++
	}
}

func (w *Window) MoveVertical(y int) {
	if y > w.ymaxoffset {
		y = w.ymaxoffset
	} else if y < 0 {
		y = 0
	}

	w.yoffset = y
}

func (w *Window) MoveHorizontal(x int) {
	if x > w.xmaxoffset {
		x = w.xmaxoffset
	} else if x < 0 {
		x = 0
	}

	w.xoffset = x
}

func (w *Window) MoveEndLine() {
	w.MoveHorizontal(w.xmaxoffset)
}

func (w *Window) MoveStartLine() {
	w.MoveHorizontal(0)
}

func (w *Window) MoveEndFile() {
	w.MoveVertical(w.ymaxoffset)
}

func (w *Window) MoveStartFile() {
	w.MoveVertical(0)
}

func (w *Window) Resize(width, height, xstart, ystart int) error {
	w.width = width
	w.height = height
	w.xstart = xstart
	w.ystart = ystart

	if err := w.scanInput(); err != nil {
		return err
	}

	return nil
}

// TODO should create cells so that they are drawable already
// then Draw shoul just take viewable cells and print them
func (w *Window) scanInput() error {
	var err error
	var c rune
	var currX int
	columns, rows := 0, 0
	view := bytes.NewBuffer(w.buffer.Bytes())
	for i := 0; ; i++ {
		if c, _, err = view.ReadRune(); err != nil {
			break
		}

		switch c {
		case '\n':
			rows++
			if currX > columns {
				columns = currX
			}
			currX = 0
		case '\t':
			currX += w.tabspaces
		default:
			currX++
		}

		// collect x, y coordinates
		prev := w.cells[i]
		w.cells[i] = buffer.Cell{FG: prev.FG, BG: prev.BG, X: currX, Y: rows, Idx: i}
	}

	if rows <= w.height {
		w.ymaxoffset = 0
	} else {
		w.ymaxoffset = rows - w.height
	}

	if w.wrap {
		w.xmaxoffset = 0
	} else if columns >= w.width {
		w.xmaxoffset = columns - w.width
	} else {
		w.xmaxoffset = 0
	}

	if err != io.EOF {
		return err
	}

	return nil
}

// Draw returns number of lines written
func (w *Window) Draw() error {
	var r rune
	var err error
	x := w.xstart
	y := w.ystart
	xoffset := w.xoffset
	yoffset := w.yoffset
	view := bytes.NewBuffer(w.buffer.Bytes())

	// draw until we've filled all available cells
	for i := 0; y-w.ystart < w.height; i++ {
		if r, _, err = view.ReadRune(); err != nil {
			break
		}

		// wrap or skip content
		if x-w.xstart == w.width {
			if w.wrap {
				y++
				x = w.xstart
			} else {
				if r == '\n' {
					y++
					x = w.xstart
					xoffset = w.xoffset
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
				x = w.xstart
				xoffset = w.xoffset
			}
		case '\t':
			if yoffset != 0 {
				continue
			}
			if xoffset <= 0 {
				x += w.tabspaces
			} else {
				xoffset -= w.tabspaces
			}
		default:
			if yoffset != 0 {
				continue
			}
			if xoffset > 0 {
				xoffset--
				continue
			}
			c := w.cells[i]
			termbox.SetCell(x, y, r, c.FG, c.BG)
			x++
		}
	}

	if err == io.EOF {
		return nil
	}

	return err
}
