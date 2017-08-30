package fractal

import (
	"bytes"
	"fmt"
	"io"

	"termbox"
)

// TODO add alignment
type Scroll struct {
	Buffer
	Wrap      bool              // lines longer than the width of the window will wrap and displaying continues on the next line. wrap text
	Tabspaces int               // number of spaces to use when expanding tabs
	ResultsFG termbox.Attribute // foreground attribute for search results
	ResultsBG termbox.Attribute // background attribute for search results
	cells     []Cell
	maxoffset Coordinates
	offset    Coordinates
	position  Coordinates
	width     int
	height    int
}

func (w *Scroll) Init() {
	w.cells = make([]Cell, 0)

	w.Tabspaces = 4
	w.ResultsFG, w.ResultsBG = termbox.AttrReverse, termbox.AttrReverse
}

func (w *Scroll) YOffset() int {
	return w.offset.Y
}

func (w *Scroll) XOffset() int {
	return w.offset.X
}

func (w *Scroll) CanSeekUp() bool {
	return w.offset.Y > 0
}

func (w *Scroll) CanSeekDown() bool {
	return w.offset.Y < w.maxoffset.Y
}

func (w *Scroll) CanSeekLeft() bool {
	return w.offset.X > 0
}

func (w *Scroll) CanSeekRight() bool {
	return w.offset.X < w.maxoffset.X
}

func (w *Scroll) SeekUp() {
	if w.CanSeekUp() {
		w.offset.Y--
	}
}

func (w *Scroll) SeekDown() {
	if w.CanSeekDown() {
		w.offset.Y++
	}
}

func (w *Scroll) SeekLeft() {
	if w.CanSeekLeft() {
		w.offset.X--
	}
}

func (w *Scroll) SeekRight() {
	if w.CanSeekRight() {
		w.offset.X++
	}
}

func (w *Scroll) SeekVertical(y int) {
	if y > w.maxoffset.Y {
		y = w.maxoffset.Y
	} else if y < 0 {
		y = 0
	}

	w.offset.Y = y
}

func (w *Scroll) SeekHorizontal(x int) {
	if x > w.maxoffset.X {
		x = w.maxoffset.X
	} else if x < 0 {
		x = 0
	}

	w.offset.X = x
}

func (w *Scroll) SeekEndLine() {
	w.SeekHorizontal(w.maxoffset.X)
}

func (w *Scroll) SeekStartLine() {
	w.SeekHorizontal(0)
}

func (w *Scroll) SeekEndFile() {
	w.SeekVertical(w.maxoffset.Y)
}

func (w *Scroll) SeekStartFile() {
	w.SeekVertical(0)
}

func (w *Scroll) moveResult(i int) {
	res := w.cells[i]

	w.SeekVertical(res.Y)

	if res.X >= w.offset.X+w.width {
		// move to the minimal x to render search result
		runes := []rune(string(w.Buffer.SearchText()))
		w.SeekHorizontal(res.X - w.width + len(runes))
	} else if res.X < w.offset.X {
		w.SeekHorizontal(res.X)
	}
}

func (w *Scroll) SeekNextResult() {
	i, ok := w.Buffer.NextResult()

	if !ok {
		return
	}

	w.moveResult(i)
}

func (w *Scroll) SeekPrevResult() {
	i, ok := w.Buffer.PrevResult()

	if !ok {
		return
	}

	w.moveResult(i)
}

func (w *Scroll) Position() (x, y int) {
	return w.position.X, w.position.Y
}

func (w *Scroll) Move(x, y int) error {
	w.position.X = x
	w.position.Y = y
	return nil
}

func (w *Scroll) Resize(width, height int) error {
	w.width = width
	w.height = height

	if err := w.scan(); err != nil {
		return err
	}

	return nil
}

func (w *Scroll) Height() int {
	return w.height
}

func (w *Scroll) Width() int {
	return w.width
}

func reserve(s []Cell, capacity int) []Cell {
	slen := len(s)

	if capacity <= len(s) {
		return s
	}

	if capacity <= cap(s) {
		return s[:capacity]
	}

	n := make([]Cell, capacity)
	copied := copy(n, s)
	if copied != slen {
		panic(fmt.Sprintf("copy failed to copy all cells: did=%d; should=%d", copied, slen))
	}

	return n
}

func (w *Scroll) scan() (err error) {
	w.cells = reserve(w.cells, w.Buffer.Len())
	ncells := w.cells[:0]

	var r rune
	var size int
	columns, row := 0, 0

	for view, x, i := bytes.NewBuffer(w.Buffer.Bytes()), 0, 0; ; {
		if r, size, err = view.ReadRune(); err != nil {
			break
		}

		var cell Cell

		switch r {
		case '\n':
			row++
			x = 0
			cell = Cell{}
		case '\t':
			x += w.Tabspaces
			cell = Cell{}
		default:
			cell = Cell{
				Fg: w.cells[i].Fg,
				Bg: w.cells[i].Bg,
				Ch: r,
				Coordinates: Coordinates{
					X: x,
					Y: row,
				},
			}
			x++
		}

		ncells = append(ncells, cell)

		// we want one cell per byte so that search and indexing is natural
		for ; size > 1; size-- {
			ncells = append(ncells, Cell{})
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

	if w.Wrap {
		w.maxoffset.X = 0
	} else if columns >= w.width {
		w.maxoffset.X = columns - w.width
	} else {
		w.maxoffset.X = 0
	}

	// adjust offsets in case they became ilegal
	w.SeekHorizontal(w.offset.X)
	w.SeekVertical(w.offset.Y)

	w.Buffer.MarkScanned()

	if err == io.EOF {
		return nil
	}

	return err
}

func (w *Scroll) draw(writer Writer) (err error) {
	var x, y int
	var c Cell
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

func (w *Scroll) wrapdraw(writer Writer) (err error) {
	var x, y, ywindow int
	var c Cell
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

func (w *Scroll) Draw(writer Writer) (err error) {
	if !w.Buffer.Scanned() {
		if err = w.scan(); err != nil {
			return
		}
	}
	if w.Wrap {
		return w.wrapdraw(writer)
	}

	return w.draw(writer)
}

func (w *Scroll) resetCells() {
	for i, c := range w.cells {
		w.cells[i] = Cell{
			Fg: 0,
			Bg: 0,
			Ch: c.Ch,
			Coordinates: Coordinates{
				X: c.X,
				Y: c.Y,
			},
		}
	}
}

func (w *Scroll) Search(text string) int {
	w.resetCells()
	return w.Buffer.Search([]byte(text), w.cells, w.ResultsFG, w.ResultsBG)
}

func (w *Scroll) CellAt(idx int) Cell {
	return w.cells[idx]
}
