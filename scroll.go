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

func (s *Scroll) SeekUp() (ok bool) {
	if ok = s.CanSeekUp(); ok {
		s.offset.Y--
	}
	return ok
}

func (s *Scroll) SeekDown() (ok bool) {
	if ok = s.CanSeekDown(); ok {
		s.offset.Y++
	}
	return
}

func (s *Scroll) SeekLeft() (ok bool) {
	if ok = s.CanSeekLeft(); ok {
		s.offset.X--
	}
	return
}

func (s *Scroll) SeekRight() (ok bool) {
	if ok = s.CanSeekRight(); ok {
		s.offset.X++
	}
	return
}

func (s *Scroll) SeekVertical(y int) (ok bool) {
	if max := s.getMaxYOffset(); y > max {
		y = max
	} else if y < 0 {
		y = 0
	}

	ok = s.offset.Y != y
	s.offset.Y = y

	return
}

func (s *Scroll) SeekHorizontal(x int) (ok bool) {
	if max := s.getMaxXOffset(); x > max {
		x = max
	} else if x < 0 {
		x = 0
	}

	ok = s.offset.X != x
	s.offset.X = x

	return
}

func (s *Scroll) SeekEndLine() bool {
	return s.SeekHorizontal(s.getMaxXOffset())
}

func (s *Scroll) SeekStartLine() bool {
	return s.SeekHorizontal(0)
}

func (s *Scroll) SeekEndFile() bool {
	return s.SeekVertical(s.getMaxYOffset())
}

func (s *Scroll) SeekStartFile() bool {
	return s.SeekVertical(0)
}

func (s *Scroll) seekTo(pos Coordinates, padding int) bool {
	yok := s.SeekVertical(pos.Y)
	var xok bool

	if pos.X >= s.offset.X+s.width {
		xok = s.SeekHorizontal(pos.X - s.width + padding)
	} else if pos.X < s.offset.X {
		xok = s.SeekHorizontal(pos.X)
	}

	return yok || xok
}

func (s *Scroll) SeekTo(pos Coordinates) bool {
	return s.seekTo(pos, 1)
}

func (s *Scroll) SeekNextResult() bool {
	pos, ok := s.NextResult()
	if !ok {
		return false
	}

	return s.seekTo(pos, len(s.searchText))
}

func (s *Scroll) SeekPrevResult() bool {
	pos, ok := s.PrevResult()
	if !ok {
		return false
	}

	return s.seekTo(pos, len(s.searchText))
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

func (s *Scroll) getView() [][]termbox.Cell {
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
		y = rows - s.height - 1
	}
	return
}

// TODO optimize for large files
func (s *Scroll) draw(writer Writer) (err error) {
	xwindow := s.offset.X + s.width
	ywindow := s.offset.Y + s.height
	for y, r := range s.cells {
		for x, c := range r {
			if c.Ch != 0 && y >= s.offset.Y && y < ywindow && x >= s.offset.X && x < xwindow {
				xi := x - s.offset.X + s.position.X
				yi := y - s.offset.Y + s.position.Y
				if err = writer.Write(xi, yi, c.Ch, c.Fg, c.Bg); err != nil {
					return
				}
			}
		}
	}

	return nil
}

func (s *Scroll) wrapdraw(writer Writer) (err error) {
	var xi, yi, ywindow int
	xwindow := s.width
	wraps := 0
	for y, r := range s.cells {
		for x, c := range r {
			ywindow = s.offset.Y + s.height - wraps
			if c.Ch != 0 && y >= s.offset.Y && y < ywindow {
				xi = x
				if xi >= xwindow {
					for xi >= xwindow {
						xi -= xwindow
					}
					if xi == 0 {
						wraps++
					}
				}
				yi = y - s.offset.Y + s.position.Y + wraps
				if err = writer.Write(xi+s.position.X, yi, c.Ch, c.Fg, c.Bg); err != nil {
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

	slen := len(s.searchText)
	if slen == 0 {
		return 0
	}

	var pos Coordinates
	var o int
	for y, r := range s.cells {
		for x, c := range r {
			if c.Ch != s.searchText[o] {
				o = 0
			} else if o == 0 {
				pos = Coordinates{X: x, Y: y}
				o++
			} else {
				o++
			}
			if o == slen {
				s.reslist.PushBack(pos)
				lX := pos.X + slen
				for j := pos.X; j < lX; j++ {
					s.SetAttr(Coordinates{X: j, Y: pos.Y}, s.ResultsFG, s.ResultsBG)
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
func (s *Scroll) RowLastIdx(y int) (x int, ok bool) {
	if y < 0 {
		panic("illegal index")
	}
	if y < len(s.cells) {
		ok = true
		if len := len(s.cells[y]); len > 0 {
			x = len - 1
		}
	}

	return
}

func (s *Scroll) Rows() int {
	return s.rows
}

func (s *Scroll) Columns() int {
	return s.columns
}
