package fractal

import (
	"container/list"

	"termbox"
)

// TODO add alignment
type Scroll struct {
	CellBuf
	width, height int
	columns, rows int
	searchText    []rune
	offset        Coordinates
	position      Coordinates
	reslist       list.List
	result        *list.Element
	ResultsFG     termbox.Attribute // foreground attribute for search results
	ResultsBG     termbox.Attribute // background attribute for search results
	Wrap          bool              // lines longer than the width of the window will wrap and displaying continues on the next line. wrap text
}

func (s *Scroll) Reset() {
	s.columns, s.rows = 0, 0
	s.result = nil
	s.searchText = nil
	s.reslist.Init()
	s.CellBuf.Init()
}

func NewScroll() (s *Scroll) {
	s = new(Scroll)
	s.Init()
	return
}

func (s *Scroll) Init() {
	s.ResultsFG, s.ResultsBG = termbox.AttrReverse, termbox.AttrReverse
	s.Reset()
}

func (s *Scroll) CanSeekUp() bool {
	return s.offset.Y > 0
}

func (s *Scroll) CanSeekDown() bool {
	return s.offset.Y < s.getMaxYOffset()
}

func (s *Scroll) CanSeekLeft() bool {
	return s.offset.X > 0
}

func (s *Scroll) CanSeekRight() bool {
	return s.offset.X < s.getMaxXOffset()
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
	if max := s.getMaxYOffset(); y > max {
		y = max
	} else if y < 0 {
		y = 0
	}

	s.offset.Y = y
}

func (s *Scroll) SeekHorizontal(x int) {
	if max := s.getMaxXOffset(); x > max {
		x = max
	} else if x < 0 {
		x = 0
	}

	s.offset.X = x
}

func (s *Scroll) SeekEndLine() {
	s.SeekHorizontal(s.getMaxXOffset())
}

func (s *Scroll) SeekStartLine() {
	s.SeekHorizontal(0)
}

func (s *Scroll) SeekEndFile() {
	s.SeekVertical(s.getMaxYOffset())
}

func (s *Scroll) SeekStartFile() {
	s.SeekVertical(0)
}

func (s *Scroll) seekTo(pos Coordinates, padding int) {
	s.SeekVertical(pos.Y)

	if pos.X >= s.offset.X+s.width {
		s.SeekHorizontal(pos.X - s.width + padding)
	} else if pos.X < s.offset.X {
		s.SeekHorizontal(pos.X)
	}
}

func (s *Scroll) SeekTo(pos Coordinates) {
	s.seekTo(pos, 1)
}

func (s *Scroll) SeekNextResult() {
	pos, ok := s.NextResult()
	if !ok {
		return
	}

	s.seekTo(pos, len(s.searchText))
}

func (s *Scroll) SeekPrevResult() {
	pos, ok := s.PrevResult()
	if !ok {
		return
	}

	s.seekTo(pos, len(s.searchText))
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

func (s *Scroll) getView() [][]Cell {
	ywindow := s.offset.Y + s.height
	return s.cells[s.offset.Y:ywindow]
}

func (s *Scroll) getMaxXOffset() (x int) {
	view := s.getView()
	columns := 0
	for _, r := range view {
		if l := len(r); l > columns {
			columns = l
		}
	}
	if s.Wrap {
		x = 0
	} else if columns >= s.width {
		x = columns - s.width
	} else {
		x = 0
	}
	return
}

func (s *Scroll) getMaxYOffset() (y int) {
	rows := len(s.cells)
	if rows <= s.height {
		y = 0
	} else {
		y = rows - s.height
	}
	return
}

func (s *Scroll) draw(writer Writer) (err error) {
	xwindow := s.offset.X + s.width
	ywindow := s.offset.Y + s.height
	for _, r := range s.cells {
		for _, c := range r {
			if c.Ch != 0 && c.Y >= s.offset.Y && c.Y < ywindow && c.X >= s.offset.X && c.X < xwindow {
				x := c.X - s.offset.X + s.position.X
				y := c.Y - s.offset.Y + s.position.Y
				if err = writer.Write(x, y, c.Ch, c.Fg, c.Bg); err != nil {
					return
				}
			}
		}
	}

	return nil
}

func (s *Scroll) wrapdraw(writer Writer) (err error) {
	var x, y, ywindow int
	xwindow := s.width
	wraps := 0
	for _, r := range s.cells {
		for _, c := range r {
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
	}

	return nil
}

func (s *Scroll) Draw(writer Writer) (err error) {
	if s.Wrap {
		return s.wrapdraw(writer)
	}

	return s.draw(writer)
}

func (s *Scroll) resetCells() {
	for y, r := range s.cells {
		for x := range r {
			s.cells[y][x].Fg, s.cells[y][x].Bg = 0, 0
		}
	}
}

func (s *Scroll) SetAttr(pos Coordinates, fg, bg termbox.Attribute) {
	s.cells[pos.Y][pos.X].Bg = bg
	s.cells[pos.Y][pos.X].Fg = fg
}

func (s *Scroll) Search(text string) int {
	s.resetCells()
	s.reslist.Init()
	s.result = nil
	s.searchText = []rune(text)

	m := len(s.searchText)
	if m == 0 {
		return 0
	}

	var i Cell
	var o int
	for _, r := range s.cells {
		for _, c := range r {
			if c.Ch != s.searchText[o] {
				o = 0
			} else if o == 0 {
				i = c
				o++
			} else {
				o++
			}
			if o == m {
				s.reslist.PushBack(i.Coordinates)
				for _, cell := range s.cells[i.Y][i.X : c.X+1] {
					s.SetAttr(cell.Coordinates, s.ResultsFG, s.ResultsBG)
				}
				o = 0
			}
		}
		// search is not performed across rows
		o = 0
	}

	return s.reslist.Len()
}

func (s *Scroll) PrevResult() (pos Coordinates, ok bool) {
	if s.result == nil {
		s.result = s.reslist.Back()
	} else if s.result = s.result.Prev(); s.result == nil {
		s.result = s.reslist.Back()
	}

	if s.result == nil {
		return
	}

	pos = s.result.Value.(Coordinates)
	ok = true
	return
}

func (s *Scroll) NextResult() (pos Coordinates, ok bool) {
	if s.result == nil {
		s.result = s.reslist.Front()
	} else if s.result = s.result.Next(); s.result == nil {
		s.result = s.reslist.Front()
	}

	if s.result == nil {
		return
	}

	pos = s.result.Value.(Coordinates)
	ok = true
	return
}

// Result returns the current search result's coordinates
func (s *Scroll) Result() (pos Coordinates, ok bool) {
	if s.result == nil {
		ok = false
		return
	}
	pos = s.result.Value.(Coordinates)
	ok = true
	return
}

// RowLastIdx returns the width of row i or panics if row i does not exist
func (s *Scroll) RowLastIdx(i int) (int, bool) {
	if i < 0 {
		panic("illegal index")
	}
	if i < len(s.cells) {
		return len(s.cells[i]), true
	}

	return 0, false
}

func (s *Scroll) Rows() int {
	return s.rows
}

func (s *Scroll) Columns() int {
	return s.columns
}
