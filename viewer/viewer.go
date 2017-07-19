package viewer

import (
	"bytes"
	"io"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/buffer"
)

type Viewer struct {
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
	Tabspaces  int
}

func (w *Viewer) Init(initial *buffer.Buffer) {
	w.cells = make(map[int]buffer.Cell)
	w.Tabspaces = 4
	w.buffer = initial
}

func New(initial *buffer.Buffer) *Viewer {
	w := new(Viewer)
	w.Init(initial)
	return w
}

func (w *Viewer) Cells() map[int]buffer.Cell {
	return w.cells
}

func (w *Viewer) YOffset() int {
	return w.yoffset
}

func (w *Viewer) XOffset() int {
	return w.xoffset
}

func (w *Viewer) CanMoveUp() bool {
	return w.yoffset > 0
}

func (w *Viewer) CanMoveDown() bool {
	return w.yoffset < w.ymaxoffset
}

func (w *Viewer) CanMoveLeft() bool {
	return w.xoffset > 0
}

func (w *Viewer) CanMoveRight() bool {
	return w.xoffset < w.xmaxoffset
}

func (w *Viewer) MoveUp() {
	if w.CanMoveUp() {
		w.yoffset--
	}
}

func (w *Viewer) MoveDown() {
	if w.CanMoveDown() {
		w.yoffset += 1
	}
}

func (w *Viewer) MoveLeft() {
	if w.CanMoveLeft() {
		w.xoffset--
	}
}

func (w *Viewer) MoveRight() {
	if w.CanMoveRight() {
		w.xoffset++
	}
}

func (w *Viewer) MoveVertical(y int) {
	if y > w.ymaxoffset {
		y = w.ymaxoffset
	} else if y < 0 {
		y = 0
	}

	w.yoffset = y
}

func (w *Viewer) MoveHorizontal(x int) {
	if x > w.xmaxoffset {
		x = w.xmaxoffset
	} else if x < 0 {
		x = 0
	}

	w.xoffset = x
}

func (w *Viewer) MoveEndLine() {
	w.MoveHorizontal(w.xmaxoffset)
}

func (w *Viewer) MoveStartLine() {
	w.MoveHorizontal(0)
}

func (w *Viewer) MoveEndFile() {
	w.MoveVertical(w.ymaxoffset)
}

func (w *Viewer) MoveStartFile() {
	w.MoveVertical(0)
}

func (w *Viewer) Position() (x, y int) {
	return w.xstart, w.ystart
}

func (w *Viewer) MoveTo(x, y int) error {
	w.xstart = x
	w.ystart = y
	return nil
}

func (w *Viewer) Resize(width, height int) error {
	w.width = width
	w.height = height

	if err := w.scanInput(); err != nil {
		return err
	}

	return nil
}

func (w *Viewer) Height() int {
	return w.height
}

func (w *Viewer) Width() int {
	return w.width
}

// TODO should create cells so that they are drawable already
// then Draw shoul just take viewable cells and print them
func (w *Viewer) scanInput() error {
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
			currX += w.Tabspaces
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

func (w *Viewer) SetBuffer(buf *buffer.Buffer) (orig *buffer.Buffer, err error) {
	orig = w.buffer
	w.buffer = buf

	if err = w.scanInput(); err != nil {
		return
	}

	return
}

func (w *Viewer) Draw(writer fractal.CellWriter) error {
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
				x += w.Tabspaces
			} else {
				xoffset -= w.Tabspaces
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
			writer.SetCell(x, y, r, c.FG, c.BG)
			x++
		}
	}

	if err == io.EOF {
		return nil
	}

	return err
}
