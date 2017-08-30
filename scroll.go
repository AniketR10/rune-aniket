package fractal

import (
	"container/list"
	"fmt"
	"io"

	"termbox"
)

// TODO add alignment
type Scroll struct {
	buffer     []rune
	cells      []Cell
	scanned    bool
	maxoffset  Coordinates
	offset     Coordinates
	position   Coordinates
	width      int
	height     int
	reslist    list.List
	result     *list.Element
	searchText []rune
	Wrap       bool              // lines longer than the width of the window will wrap and displaying continues on the next line. wrap text
	Tabspaces  int               // number of spaces to use when expanding tabs
	ResultsFG  termbox.Attribute // foreground attribute for search results
	ResultsBG  termbox.Attribute // background attribute for search results
}

func (w *Scroll) Reset() {
	w.cells = w.cells[:0]
	w.buffer = w.buffer[:0]
	w.reslist.Init()
	w.result = nil
	w.searchText = nil
}

func (w *Scroll) Init() {
	w.cells = make([]Cell, 0)
	w.Tabspaces = 4
	w.ResultsFG, w.ResultsBG = termbox.AttrReverse, termbox.AttrReverse
	w.Reset()
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
		w.SeekHorizontal(res.X - w.width + len(w.searchText))
	} else if res.X < w.offset.X {
		w.SeekHorizontal(res.X)
	}
}

func (w *Scroll) SeekNextResult() {
	i, ok := w.NextResult()

	if !ok {
		return
	}

	w.moveResult(i)
}

func (w *Scroll) SeekPrevResult() {
	i, ok := w.PrevResult()

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
	w.cells = reserve(w.cells, len(w.buffer))
	ncells := w.cells[:0]

	columns, row := 0, 0
	x := 0

	for i, r := range w.buffer {
		var cell Cell

		switch r {
		case '\n':
			row++
			x = 0
		case '\t':
			x += w.Tabspaces
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

		if x > columns {
			columns = x
		}
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

	// adjust offsets in case they became illegal
	w.SeekHorizontal(w.offset.X)
	w.SeekVertical(w.offset.Y)

	w.scanned = true

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
	if !w.scanned {
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

// we cannot use the optimized byte or string search routines in std
// since we need to know the index of the rune as it is drawn in the screen grid
func index(s, sep []rune) int {
	n, m := len(s), len(sep)
	if m == 0 {
		return 0
	}
	if m > n {
		return -1
	}

	var i, o int
	for i = 0; i < n && o < m; i++ {
		if s[i] != sep[o] {
			o = 0
		} else {
			o++
		}
	}

	if o == m {
		return i - o
	}

	return -1
}

func (w *Scroll) Search(text string) int {
	w.resetCells()
	w.reslist.Init()
	w.result = nil
	w.searchText = []rune(text)

	tlen := len(w.searchText)
	if tlen == 0 {
		return 0
	}

	view := w.buffer

	var a, i int
	for {
		if i = index(view, w.searchText); i == -1 {
			break
		}

		// use anchor to translate index to original slice
		a += i

		for j, last := a, a+tlen; j < last; j++ {
			w.cells[j] = Cell{
				Fg: w.ResultsFG,
				Bg: w.ResultsBG,
				Ch: w.cells[j].Ch,
				Coordinates: Coordinates{
					X: w.cells[j].X,
					Y: w.cells[j].Y,
				},
			}
		}

		// mark first fractal.Cell as result index
		w.reslist.PushBack(a)

		view = view[i+tlen:]

		// set next anchor
		a += tlen
	}

	return w.reslist.Len()
}

func (w *Scroll) PrevResult() (int, bool) {
	if w.result == nil {
		w.result = w.reslist.Back()
	} else if w.result = w.result.Prev(); w.result == nil {
		w.result = w.reslist.Back()
	}

	if w.result == nil {
		return 0, false
	}

	return w.result.Value.(int), true
}

func (w *Scroll) NextResult() (int, bool) {
	if w.result == nil {
		w.result = w.reslist.Front()
	} else if w.result = w.result.Next(); w.result == nil {
		w.result = w.reslist.Front()
	}

	if w.result == nil {
		return 0, false
	}

	return w.result.Value.(int), true
}

func (w *Scroll) CellAt(idx int) Cell {
	return w.cells[idx]
}

func (s *Scroll) Write(p string) (n int, err error) {
	s.scanned = false
	s.buffer = append(s.buffer, []rune(p)...)
	return len(p), nil
}
func (s *Scroll) WriteRune(r rune) (n int, err error) {
	s.scanned = false
	s.buffer = append(s.buffer, r)
	return 1, nil
}

func (s *Scroll) WriteAt(pos Coordinates, r rune) (n int, err error) {
	s.scanned = false
	panic("todo")
}

func (s *Scroll) Truncate(n int) {
	s.scanned = false
	s.buffer = s.buffer[:n]
}

func (s *Scroll) TruncateAt(pos Coordinates) error {
	s.scanned = false
	panic("todo")
}

func (s *Scroll) Len() int {
	return len(s.buffer)
}

func (s *Scroll) String() string {
	return string(s.buffer)
}
