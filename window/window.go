package window

import (
	"bytes"
	"fmt"
	"io"

	"github.com/ernestrc/fractal"
)

type Window struct {
	buffer    *Buffer             // content buffer
	cells     []fractal.Cell      // mapping from content index to x, y coordinates
	wrap      bool                // wrap text
	maxoffset fractal.Coordinates // max content offsets
	offset    fractal.Coordinates // content offsets
	position  fractal.Coordinates // position in the underlying writer
	width     int                 // window width
	height    int                 // window height
	Tabspaces int
}

func (w *Window) Init(initial *Buffer, width, height int, tabspaces int, wrap bool) {
	w.Tabspaces = tabspaces
	w.wrap = wrap
	w.buffer = initial
	w.width, w.height = width, height
	if initial != nil {
		w.cells = make([]fractal.Cell, initial.Len())
	} else {
		w.cells = make([]fractal.Cell, 0)
	}
}

func New(initial *Buffer, width, height int, tabspaces int, wrap bool) *Window {
	w := new(Window)
	w.Init(initial, width, height, tabspaces, wrap)
	return w
}

func (w *Window) YOffset() int {
	return w.offset.Y
}

func (w *Window) XOffset() int {
	return w.offset.X
}

func (w *Window) CanSeekUp() bool {
	return w.offset.Y > 0
}

func (w *Window) CanSeekDown() bool {
	return w.offset.Y < w.maxoffset.Y
}

func (w *Window) CanSeekLeft() bool {
	return w.offset.X > 0
}

func (w *Window) CanSeekRight() bool {
	return w.offset.X < w.maxoffset.X
}

func (w *Window) SeekUp() {
	if w.CanSeekUp() {
		w.offset.Y--
	}
}

func (w *Window) SeekDown() {
	if w.CanSeekDown() {
		w.offset.Y++
	}
}

func (w *Window) SeekLeft() {
	if w.CanSeekLeft() {
		w.offset.X--
	}
}

func (w *Window) SeekRight() {
	if w.CanSeekRight() {
		w.offset.X++
	}
}

func (w *Window) SeekVertical(y int) {
	if y > w.maxoffset.Y {
		y = w.maxoffset.Y
	} else if y < 0 {
		y = 0
	}

	w.offset.Y = y
}

func (w *Window) SeekHorizontal(x int) {
	if x > w.maxoffset.X {
		x = w.maxoffset.X
	} else if x < 0 {
		x = 0
	}

	w.offset.X = x
}

func (w *Window) SeekEndLine() {
	w.SeekHorizontal(w.maxoffset.X)
}

func (w *Window) SeekStartLine() {
	w.SeekHorizontal(0)
}

func (w *Window) SeekEndFile() {
	w.SeekVertical(w.maxoffset.Y)
}

func (w *Window) SeekStartFile() {
	w.SeekVertical(0)
}

func (w *Window) moveResult(i int) {

	res := w.cells[i]

	w.SeekVertical(res.Y)

	if res.X >= w.offset.X+w.width {
		// move to the minimal x to render search result
		w.SeekHorizontal(res.X - w.width + len([]rune(string(w.buffer.searchText))))
	} else if res.X < w.offset.X {
		w.SeekHorizontal(res.X)
	}
}

func (w *Window) SeekNextResult() {
	i, ok := w.buffer.nextResult()

	if !ok {
		return
	}

	w.moveResult(i)
}

func (w *Window) SeekPrevResult() {
	i, ok := w.buffer.prevResult()

	if !ok {
		return
	}

	w.moveResult(i)
}

func (w *Window) Position() (x, y int) {
	return w.position.X, w.position.Y
}

func (w *Window) Move(x, y int) {
	w.position.X = x
	w.position.Y = y
}

func (w *Window) Resize(width, height int) error {
	w.width = width
	w.height = height

	if err := w.Scan(); err != nil {
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

func (w *Window) cell(idx int) fractal.Cell {
	return w.cells[idx]
}

func reserve(s []fractal.Cell, capacity int) []fractal.Cell {
	slen := len(s)

	if capacity <= len(s) {
		return s
	}

	if capacity <= cap(s) {
		return s[:capacity]
	}

	n := make([]fractal.Cell, capacity)
	copied := copy(n, s)
	if copied != slen {
		panic(fmt.Sprintf("copy failed to copy all cells: did=%d; should=%d", copied, slen))
	}

	return n
}

func (w *Window) Scan() (err error) {
	if w.buffer == nil {
		w.maxoffset.Y = 0
		w.maxoffset.X = 0
		return nil
	}

	w.cells = reserve(w.cells, w.buffer.Len())
	ncells := w.cells[:0]

	var r rune
	var size int
	columns, row := 0, 0

	for view, x, i := bytes.NewBuffer(w.buffer.Bytes()), 0, 0; ; {
		if r, size, err = view.ReadRune(); err != nil {
			break
		}

		var cell fractal.Cell

		switch r {
		case '\n':
			row++
			x = 0
			cell = fractal.Cell{}
		case '\t':
			x += w.Tabspaces
			cell = fractal.Cell{}
		default:
			cell = fractal.Cell{
				Fg: w.cells[i].Fg,
				Bg: w.cells[i].Bg,
				Ch: r,
				Coordinates: fractal.Coordinates{
					X: x,
					Y: row,
				},
			}
			x++
		}

		ncells = append(ncells, cell)

		// we want one cell per byte so that search and indexing is natural
		for ; size > 1; size-- {
			ncells = append(ncells, fractal.Cell{})
		}

		if x > columns {
			columns = x
		}

		i += size
	}

	w.cells = ncells

	rows := row + 1 // row is index starting at 0

	// adjust max content offsets
	if rows <= w.height {
		w.maxoffset.Y = 0
	} else {
		w.maxoffset.Y = rows - w.height
	}

	if w.wrap {
		w.maxoffset.X = 0
	} else if columns >= w.width {
		w.maxoffset.X = columns - w.width
	} else {
		w.maxoffset.X = 0
	}

	// adjust offsets in case they became ilegal
	w.SeekHorizontal(w.offset.X)
	w.SeekVertical(w.offset.Y)

	if err == io.EOF {
		return nil
	}

	return err
}

func (w *Window) SetBuffer(buf *Buffer) (orig *Buffer, err error) {
	orig = w.buffer
	w.buffer = buf

	if err = w.Scan(); err != nil {
		return
	}

	return
}

func (w *Window) draw(writer fractal.Writer) (err error) {
	var x, y int
	var c fractal.Cell
	xwindow := w.offset.X + w.width
	ywindow := w.offset.Y + w.height
	for _, c = range w.cells {
		if c.Ch != 0 && c.Y >= w.offset.Y && c.Y < ywindow && c.X >= w.offset.X && c.X < xwindow {
			x = c.X - w.offset.X + w.position.X
			y = c.Y - w.offset.Y + w.position.Y
			if err = writer.Write(x, y, c.Ch, c.Fg, c.Bg); err != nil {
				return
			}
		}
	}

	return nil
}

func (w *Window) wrapdraw(writer fractal.Writer) (err error) {
	var x, y, ywindow int
	var c fractal.Cell
	xwindow := w.width
	wraps := 0
	for _, c = range w.cells {
		ywindow = w.offset.Y + w.height - wraps
		if c.Ch != 0 && c.Y >= w.offset.Y && c.Y < ywindow {
			x = c.X
			if x >= xwindow {
				for ; x >= xwindow; x -= xwindow {
				}
				if x == 0 {
					wraps++
				}
			}
			y = c.Y - w.offset.Y + w.position.Y + wraps
			if err = writer.Write(x+w.position.X, y, c.Ch, c.Fg, c.Bg); err != nil {
				return
			}
		}
	}

	return nil
}

func (w *Window) Draw(writer fractal.Writer) (err error) {
	if w.wrap {
		return w.wrapdraw(writer)
	}

	return w.draw(writer)
}

func (w *Window) resetCells() {
	for i, c := range w.cells {
		w.cells[i] = fractal.Cell{
			Fg: 0,
			Bg: 0,
			Ch: c.Ch,
			Coordinates: fractal.Coordinates{
				X: c.X,
				Y: c.Y,
			},
		}
	}
}

func (w *Window) Search(text string) int {
	w.resetCells()
	w.buffer.search([]byte(text), w.cells)
	return w.buffer.reslist.Len()
}
