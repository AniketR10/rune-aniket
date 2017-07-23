package viewer

import (
	"bytes"
	"io"

	"github.com/ernestrc/fractal"
)

type Viewer struct {
	buffer     *Buffer              // content buffer
	cells      map[int]fractal.Cell // FIXME mapping from content index to x, y coordinates
	wrap       bool                 // wrap text
	xmaxoffset int                  // max x content offset
	ymaxoffset int                  // max y content offset
	xoffset    int                  // current x content offset
	yoffset    int                  // current y content offset
	x          int                  // x offset from root window
	y          int                  // y offset from root window
	width      int                  // window width
	height     int                  // window height
	Tabspaces  int
}

func (w *Viewer) Init(initial *Buffer, width, height int, tabspaces int, wrap bool) {
	w.Tabspaces = tabspaces
	w.wrap = wrap
	w.buffer = initial
	w.width, w.height = width, height
	w.cells = make(map[int]fractal.Cell)
}

func New(initial *Buffer, width, height int, tabspaces int, wrap bool) *Viewer {
	w := new(Viewer)
	w.Init(initial, width, height, tabspaces, wrap)
	return w
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
		w.yoffset++
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

func (w *Viewer) moveResult(i int) {

	res := w.cells[i]

	w.MoveVertical(res.Y)

	if res.X >= w.xoffset+w.width {
		// move to the minimal x to render search result
		w.MoveHorizontal(res.X - w.width + len(w.buffer.searchText))
	} else if res.X < w.xoffset {
		w.MoveHorizontal(res.X)
	}
}

func (w *Viewer) MoveNextResult() {
	i, ok := w.buffer.nextResult()

	if !ok {
		return
	}

	w.moveResult(i)
}

func (w *Viewer) MovePrevResult() {
	i, ok := w.buffer.prevResult()

	if !ok {
		return
	}

	w.moveResult(i)
}

func (w *Viewer) Position() (x, y int) {
	return w.x, w.y
}

func (w *Viewer) SetPosition(x, y int) {
	w.x = x
	w.y = y
}

func (w *Viewer) Resize(width, height int) error {
	w.width = width
	w.height = height

	// avoid ending up in an illegal state
	w.xoffset, w.yoffset = 0, 0

	if err := w.Scan(); err != nil {
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

func (w *Viewer) cell(idx int) fractal.Cell {
	return w.cells[idx]
}

// TODO add max width/height drawing and remove from Draw
// so draw doesn't need to iterate the input again
func (w *Viewer) Scan() error {
	if w.buffer == nil {
		w.ymaxoffset = 0
		w.xmaxoffset = 0
		return nil
	}

	var err error
	var c rune
	var currX int
	columns, row := 0, 0
	view := bytes.NewBuffer(w.buffer.Bytes())
	for i := 0; ; i++ {
		if c, _, err = view.ReadRune(); err != nil {
			break
		}

		switch c {
		case '\n':
			row++
			currX = 0
		case '\t':
			currX += w.Tabspaces
		default:
			w.cells[i] = fractal.Cell{
				X:  currX,
				Y:  row,
				Fg: w.cells[i].Fg,
				Bg: w.cells[i].Bg,
				Ch: c,
			}
			currX++
		}
		if currX > columns {
			columns = currX
		}
	}

	rows := row + 1 // row is index starting at 0

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

func (w *Viewer) SetBuffer(buf *Buffer) (orig *Buffer, err error) {
	orig = w.buffer
	w.buffer = buf

	if err = w.Scan(); err != nil {
		return
	}

	return
}

func (w *Viewer) Draw(writer fractal.Writer) (err error) {
	var x, y int
	var c fractal.Cell
	xwindow := w.xoffset + w.width
	ywindow := w.yoffset + w.height
	for _, c = range w.cells {
		if c.Y >= w.yoffset && c.Y < ywindow && c.X >= w.xoffset && c.X < xwindow {
			x = c.X - w.xoffset + w.x
			y = c.Y - w.yoffset + w.y
			writer.SetAttributes(x, y, c.Fg, c.Bg)
			if err = writer.Write(x, y, c.Ch); err != nil {
				return
			}
		}
	}

	return nil
}

func (w *Viewer) Search(text string) int {
	w.buffer.search([]byte(text), w.cells)
	return w.buffer.reslist.Len()
}
