package window

import (
	"bytes"
	"io"

	"github.com/ernestrc/fractal/buffer"
	"github.com/ernestrc/fractal/config"
	termbox "github.com/nsf/termbox-go"
)

type Drawable interface {
	Draw() error
}

type Window struct {
	buffer     *buffer.Buffer      // content buffer
	cells      map[int]buffer.Cell // color information used for printing to window
	wrap       bool                // wrap text
	xmaxoffset int                 // max x content offset
	ymaxoffset int                 // max y content offset
	xoffset    int                 // current x content offset
	yoffset    int                 // current y content offset
	xstart     int                 // x offset from root window
	ystart     int                 // y offset from root window
	height     int                 // window height
	width      int                 // window width
	config     *config.Config
}

func New(initial *buffer.Buffer, cfg *config.Config) *Window {
	w := new(Window)
	w.cells = make(map[int]buffer.Cell)

	w.buffer = initial

	if cfg == nil {
		w.config = config.New()
	} else {
		w.config = cfg
	}

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

func (w *Window) Position() (x, y int) {
	return w.xstart, w.ystart
}

func (w *Window) Move(x, y int) {
	w.xstart = x
	w.ystart = y
}

func (w *Window) Resize(width, height int) error {
	if width == 0 {
		width = 1
	}
	if height == 0 {
		height = 1
	}

	w.width = width
	w.height = height

	if err := w.scanInput(); err != nil {
		return err
	}

	return nil
}

func (w *Window) Height() int {
	return w.height
}

func (w *Window) Width() int {
	return w.width
}

// TODO should create cells so that they are drawable already
// then Draw shoul just take viewable cells and print them
func (w *Window) scanInput() error {
	// this window does not have a buffer assigned
	if w.buffer == nil {
		w.ymaxoffset = 0
		w.xmaxoffset = 0
		return nil
	}

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
			currX += w.config.Tabspaces
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

func (w *Window) SetBuffer(buf *buffer.Buffer) (orig *buffer.Buffer, err error) {
	orig = w.buffer
	w.buffer = buf

	if err = w.scanInput(); err != nil {
		return
	}

	return
}

func (w *Window) Draw() error {
	if w.buffer == nil {
		return nil
	}

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
				x += w.config.Tabspaces
			} else {
				xoffset -= w.config.Tabspaces
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
