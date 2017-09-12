package fractal

import (
	"container/list"

	"termbox"
)

// TODO add alignment
type Scroll struct {
	buffer        []rune
	cells         []Cell
	rowwidth      []int
	width, height int
	columns, rows int
	searchText    []rune
	maxoffset     Coordinates
	offset        Coordinates
	position      Coordinates
	reslist       list.List
	result        *list.Element
	ResultsFG     termbox.Attribute // foreground attribute for search results
	ResultsBG     termbox.Attribute // background attribute for search results
	Tabspaces     int               // number of spaces to use when expanding tabs
	Wrap          bool              // lines longer than the width of the window will wrap and displaying continues on the next line. wrap text
	scanned       bool
}

func (s *Scroll) Reset() {
	s.rowwidth = s.rowwidth[:0]
	s.columns, s.rows = 0, 0
	s.cells = s.cells[:0]
	s.buffer = s.buffer[:0]
	s.reslist.Init()
	s.result = nil
	s.searchText = nil
}

func NewScroll() (s *Scroll) {
	s = new(Scroll)
	s.Init()
	return
}

func (s *Scroll) Init() {
	s.cells = make([]Cell, 0)
	s.Tabspaces = 4
	s.ResultsFG, s.ResultsBG = termbox.AttrReverse, termbox.AttrReverse
	s.Reset()
}

func (s *Scroll) CanSeekUp() bool {
	return s.offset.Y > 0
}

func (s *Scroll) CanSeekDown() bool {
	return s.offset.Y < s.maxoffset.Y
}

func (s *Scroll) CanSeekLeft() bool {
	return s.offset.X > 0
}

func (s *Scroll) CanSeekRight() bool {
	return s.offset.X < s.maxoffset.X
}

func (s *Scroll) SeekUp() {
	if s.CanSeekUp() {
		s.offset.Y--
	}
}

func (s *Scroll) SeekDown() {
	if s.CanSeekDown() {
		s.offset.Y++
	}
}

func (s *Scroll) SeekLeft() {
	if s.CanSeekLeft() {
		s.offset.X--
	}
}

func (s *Scroll) SeekRight() {
	if s.CanSeekRight() {
		s.offset.X++
	}
}

func (s *Scroll) SeekVertical(y int) {
	if y > s.maxoffset.Y {
		y = s.maxoffset.Y
	} else if y < 0 {
		y = 0
	}

	s.offset.Y = y
}

func (s *Scroll) SeekHorizontal(x int) {
	if x > s.maxoffset.X {
		x = s.maxoffset.X
	} else if x < 0 {
		x = 0
	}

	s.offset.X = x
}

func (s *Scroll) SeekEndLine() {
	s.SeekHorizontal(s.maxoffset.X)
}

func (s *Scroll) SeekStartLine() {
	s.SeekHorizontal(0)
}

func (s *Scroll) SeekEndFile() {
	s.SeekVertical(s.maxoffset.Y)
}

func (s *Scroll) SeekStartFile() {
	s.SeekVertical(0)
}

func (s *Scroll) moveResult(i int) {
	res := s.cells[i]

	s.SeekVertical(res.Y)

	if res.X >= s.offset.X+s.width {
		s.SeekHorizontal(res.X - s.width + len(s.searchText))
	} else if res.X < s.offset.X {
		s.SeekHorizontal(res.X)
	}
}

func (s *Scroll) SeekNextResult() {
	i, ok := s.NextResult()

	if !ok {
		return
	}

	s.moveResult(i)
}

func (s *Scroll) SeekPrevResult() {
	i, ok := s.PrevResult()

	if !ok {
		return
	}

	s.moveResult(i)
}

func (s *Scroll) Position() (x, y int) {
	return s.position.X, s.position.Y
}

func (s *Scroll) Move(x, y int) error {
	s.position.X = x
	s.position.Y = y
	return nil
}

func (s *Scroll) Resize(width, height int) error {
	s.width = width
	s.height = height
	s.scan()

	return nil
}

func (s *Scroll) Height() int {
	return s.height
}

func (s *Scroll) Width() int {
	return s.width
}

func reserve(s []Cell, capacity int) []Cell {
	if capacity <= len(s) {
		return s
	}

	if capacity <= cap(s) {
		return s[:capacity]
	}

	n := make([]Cell, capacity)
	copy(n, s)

	return n
}

func (s *Scroll) adjustOffsets(rows, columns int) {
	if rows <= s.height {
		s.maxoffset.Y = 0
	} else {
		s.maxoffset.Y = rows - s.height
	}

	if s.Wrap {
		s.maxoffset.X = 0
	} else if columns >= s.width {
		s.maxoffset.X = columns - s.width
	} else {
		s.maxoffset.X = 0
	}
}

// transform cells to be a rows * columns grid
// fixes tabspaces + missing cells for shorter lines
func adjustCells(cells []Cell, rows, columns int) []Cell {
	ncells := make([]Cell, rows*columns)
	for _, c := range cells {
		ncells[c.X+c.Y*columns] = c
	}

	return ncells
}

func (s *Scroll) scan() {
	s.cells = reserve(s.cells, len(s.buffer))
	ncells := s.cells[:0]
	s.rowwidth = s.rowwidth[:0]

	columns, row := 0, 0
	x := 0

	for _, r := range s.buffer {

		ncells = append(ncells, Cell{
			Ch:          r,
			Coordinates: Coordinates{X: x, Y: row},
		})
		switch r {
		case '\n':
			if x == 0 {
				s.rowwidth = append(s.rowwidth, 0)
			} else {
				s.rowwidth = append(s.rowwidth, x-1)
			}
			row++
			x = 0
		case '\t':
			x += s.Tabspaces
		default:
			x++
		}

		if x > columns {
			columns = x
		}
	}

	s.rowwidth = append(s.rowwidth, x)

	s.columns = columns
	s.rows = row + 1
	s.adjustOffsets(s.rows, s.columns)
	s.cells = adjustCells(ncells, s.rows, s.columns)

	// adjust offsets in case they became illegal
	s.SeekHorizontal(s.offset.X)
	s.SeekVertical(s.offset.Y)

	s.Search(string(s.searchText))

	s.scanned = true
}

func (s *Scroll) draw(writer Writer) (err error) {
	xwindow := s.offset.X + s.width
	ywindow := s.offset.Y + s.height
	for _, c := range s.cells {
		if c.Ch != 0 && c.Y >= s.offset.Y && c.Y < ywindow && c.X >= s.offset.X && c.X < xwindow {
			x := c.X - s.offset.X + s.position.X
			y := c.Y - s.offset.Y + s.position.Y
			if err = writer.Write(x, y, c.Ch, c.Fg, c.Bg); err != nil {
				return
			}
		}
	}

	return nil
}

func (s *Scroll) wrapdraw(writer Writer) (err error) {
	var x, y, ywindow int
	var c Cell
	xwindow := s.width
	wraps := 0
	for _, c = range s.cells {
		ywindow = s.offset.Y + s.height - wraps
		if c.Ch != 0 && c.Y >= s.offset.Y && c.Y < ywindow {
			x = c.X
			if x >= xwindow {
				for x >= xwindow {
					x -= xwindow
				}
				if x == 0 {
					wraps++
				}
			}
			y = c.Y - s.offset.Y + s.position.Y + wraps
			if err = writer.Write(x+s.position.X, y, c.Ch, c.Fg, c.Bg); err != nil {
				return
			}
		}
	}

	return nil
}

func (s *Scroll) Draw(writer Writer) (err error) {
	if !s.scanned {
		s.scan()
	}
	if s.Wrap {
		return s.wrapdraw(writer)
	}

	return s.draw(writer)
}

func (s *Scroll) resetCells() {
	for i := range s.cells {
		s.cells[i].Fg, s.cells[i].Bg = 0, 0
	}
}

// we cannot use the optimized byte or string search routines in std
// since we need to know the index of the rune as it is drawn in the screen grid
func index(s []Cell, sep []rune) int {
	m := len(sep)
	if m == 0 {
		return 0
	}

	var i, o int
	for i = 0; i < len(s) && o < m; i++ {
		if s[i].Ch != sep[o] {
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

func (s *Scroll) SetAttr(pos Coordinates, fg, bg termbox.Attribute) {
	s.setAttr(s.getIdx(pos), fg, bg)
}

func (s *Scroll) setAttr(idx int, fg, bg termbox.Attribute) {
	s.cells[idx].Fg, s.cells[idx].Bg = fg, bg
}

func (s *Scroll) Search(text string) int {
	s.resetCells()
	s.reslist.Init()
	s.result = nil
	s.searchText = []rune(text)

	tlen := len(s.searchText)
	if tlen == 0 {
		return 0
	}

	view := s.cells

	var a, i int
	for {
		if i = index(view, s.searchText); i == -1 {
			break
		}

		// use anchor to translate index to original slice
		a += i

		for j, last := a, a+tlen; j < last; j++ {
			s.setAttr(j, s.ResultsFG, s.ResultsBG)
		}

		// mark first Cell as result index
		s.reslist.PushBack(a)

		view = view[i+tlen:]

		// set next anchor
		a += tlen
	}

	return s.reslist.Len()
}

func (s *Scroll) PrevResult() (int, bool) {
	if s.result == nil {
		s.result = s.reslist.Back()
	} else if s.result = s.result.Prev(); s.result == nil {
		s.result = s.reslist.Back()
	}

	if s.result == nil {
		return 0, false
	}

	return s.result.Value.(int), true
}

func (s *Scroll) NextResult() (int, bool) {
	if s.result == nil {
		s.result = s.reslist.Front()
	} else if s.result = s.result.Next(); s.result == nil {
		s.result = s.reslist.Front()
	}

	if s.result == nil {
		return 0, false
	}

	return s.result.Value.(int), true
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

func (s *Scroll) getIdx(pos Coordinates) int {
	return pos.X + pos.Y*(s.columns+1) // +1 for newline cell
}

func (s *Scroll) WriteAt(pos Coordinates, r rune) (n int, err error) {
	s.scanned = false
	idx := s.getIdx(pos)
	s.buffer[idx] = r
	return 1, nil
}

func (s *Scroll) InsertAt(pos Coordinates, r rune) (n int, err error) {
	s.scanned = false
	idx := s.getIdx(pos)
	len := len(s.buffer)

	if idx >= len {
		s.buffer = append(s.buffer, r)
		return 1, nil
	}

	// make sure we have enough capacity
	s.buffer = append(s.buffer, 0)[:len]
	copy(s.buffer[idx+1:], s.buffer[idx:])
	s.buffer[idx] = r
	return 1, nil
}

func (s *Scroll) Truncate(n int) {
	s.scanned = false
	s.buffer = s.buffer[:n]
}

func (s *Scroll) TruncateAt(pos Coordinates) error {
	s.scanned = false
	idx := s.getIdx(pos)
	tmp := s.buffer[:idx]
	tmp = append(tmp, s.buffer[idx+1:]...)
	s.buffer = tmp
	return nil
}

// RowLastIdx returns the width of row i or panics if row i does not exist
func (s *Scroll) RowLastIdx(i int) int {
	return s.rowwidth[i]
}

func (s *Scroll) Len() int {
	return len(s.buffer)
}

func (s *Scroll) String() string {
	return string(s.buffer)
}

func (s *Scroll) Rows() int {
	return s.rows
}

func (s *Scroll) Columns() int {
	return s.columns
}
