package component

import (
	"io"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/cell"
	"github.com/ernestrc/fractal/term"
)

// Scroll adds Draw to a Buffer along with
// scrolling, searching and wrap-around capabilities.
type Scroll struct {
	buf           *cell.Buffer
	searcher      cell.Searcher
	width, height int
	searchText    []rune
	offset        term.Coordinates

	// Sets the search result attributes upon matching.
	ResultsAttr term.Attributes
	Attributes  term.Attributes

	// lines longer than the width of the scroll wrap around and
	// are rendered in the next line if Wrap is set to true.
	Wrap bool
}

func (s *Scroll) resetProps() {
	s.searchText = nil
	s.offset = term.Coordinates{}
	s.searcher.Reset()
}

// NewScroll allocates storage for a Scroll and initializes it.
func NewScroll() (s *Scroll) {
	s = new(Scroll)
	s.Init()
	return
}

// Init initializes this scroll and allocates a new cell.Buffer.
func (s *Scroll) Init() {
	buf := cell.NewBuffer()
	s.InitWithBuffer(buf)
}

// InitWithBuffer initializes this scroll with buf.
func (s *Scroll) InitWithBuffer(buf *cell.Buffer) {
	if s.ResultsAttr == (term.Attributes{}) {
		s.ResultsAttr.Fg, s.ResultsAttr.Bg = term.AttrReverse, term.AttrReverse
	}

	s.buf = buf

	// Searcher that actually performs the text search
	searcher := cell.NewSimpleSearcher(s.buf)

	attrSearcher := cell.AttrSearcher(searcher, buf, s.ResultsAttr)
	buf.Subscribe(attrSearcher)

	s.searcher = attrSearcher

	s.resetProps()
}

// CanSeekUp returns true if SeekUp would seek one row up.
func (s *Scroll) CanSeekUp() bool {
	return s.offset.Y > 0
}

// CanSeekDown returns true if SeekDown would seek one row down.
func (s *Scroll) CanSeekDown() bool {
	return s.offset.Y < s.getMaxYOffset()
}

// CanSeekLeft returns true if SeekLeft would seek one column left.
func (s *Scroll) CanSeekLeft() bool {
	return s.offset.X > 0
}

// CanSeekRight returns true if SeekRight would seek one column right.
func (s *Scroll) CanSeekRight() bool {
	return s.offset.X < s.getMaxXOffset()
}

// SeekUp shifts the contents of this scroll one row up.
func (s *Scroll) SeekUp() (ok bool) {
	if ok = s.CanSeekUp(); ok {
		s.offset.Y--
	}
	return ok
}

// SeekDown shifts the contents of this scroll one row down.
func (s *Scroll) SeekDown() (ok bool) {
	if ok = s.CanSeekDown(); ok {
		s.offset.Y++
	}
	return
}

// SeekLeft shifts the contents of this scroll one column left.
func (s *Scroll) SeekLeft() (ok bool) {
	if ok = s.CanSeekLeft(); ok {
		s.offset.X--
	}
	return
}

// SeekRight shifts the contents of this scroll one column right.
func (s *Scroll) SeekRight() (ok bool) {
	if ok = s.CanSeekRight(); ok {
		s.offset.X++
	}
	return
}

// SeekVertical shifts the contents of this scroll such that
// the vertical offset is y. If y is out of bounds the contents
// are shifted to the maximum possible y offset.
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

// SeekHorizontal shifts the contents of this scroll such that
// the horizontal offset is x. If x is out of bounds the contents
// are shifted to the maximum possible x offset.
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

// SeekEndLine shifts the contents of this scroll to the maximum x offset.
func (s *Scroll) SeekEndLine() bool {
	return s.SeekHorizontal(s.getMaxXOffset())
}

// SeekStartLine shifts the contents of this scroll to the minimum x offset.
func (s *Scroll) SeekStartLine() bool {
	return s.SeekHorizontal(0)
}

// SeekEndFile shifts the contents of this scroll to the maximum y offset.
func (s *Scroll) SeekEndFile() bool {
	return s.SeekVertical(s.getMaxYOffset())
}

// SeekStartFile shifts the contents of this scroll to the minimum y offset.
func (s *Scroll) SeekStartFile() bool {
	return s.SeekVertical(0)
}

func (s *Scroll) seekTo(pos term.Coordinates, padding int) bool {
	yok := s.SeekVertical(pos.Y)
	var xok bool

	if pos.X >= s.offset.X+s.width {
		xok = s.SeekHorizontal(pos.X - s.width + padding)
	} else if pos.X < s.offset.X {
		xok = s.SeekHorizontal(pos.X)
	}

	return yok || xok
}

// SeekTo shifts the contents of this scroll such that the offset
// is exactly at given coordinates.
func (s *Scroll) SeekTo(pos term.Coordinates) bool {
	return s.seekTo(pos, 1)
}

// SeekNextResult shifts the contents of this scroll to visualize
// the next result in the result list.
func (s *Scroll) SeekNextResult() bool {
	pos, ok := s.NextResult()
	if !ok {
		return false
	}

	return s.seekTo(pos, len(s.searchText))
}

// SeekPrevResult shifts the contents of this scroll to visualize
// the previous result in the result list.
func (s *Scroll) SeekPrevResult() bool {
	pos, ok := s.PrevResult()
	if !ok {
		return false
	}

	return s.seekTo(pos, len(s.searchText))
}

// Resize resizes this scroll to fit inside given width and height.
func (s *Scroll) Resize(width, height int) {
	s.width = width
	s.height = height
}

func (s *Scroll) getMaxXOffset() (x int) {
	const padding = 1

	view := s.buf.RawCells()[s.offset.Y:]
	columns := 0
	for _, r := range view {
		if l := len(r); l > columns {
			columns = l
		}
	}
	if s.Wrap {
		x = 0
	} else if columns >= s.width {
		x = columns - s.width + padding
	} else {
		x = 0
	}
	return
}

func (s *Scroll) getMaxYOffset() (y int) {
	rows := s.buf.Rows()
	if rows >= s.height {
		y = rows - s.height
	}
	return
}

func (s *Scroll) draw(writer fractal.Writer) {
	xwindow := s.offset.X + s.width
	ywindow := s.height
	for y, r := range s.buf.RawCells()[s.offset.Y:] {
		if y >= ywindow {
			break
		}
		for x, c := range r {
			if x >= xwindow {
				break
			}
			if c.Ch == 0 || x < s.offset.X {
				continue
			}
			xi := x - s.offset.X
			if c.Bg == 0 {
				c.Bg = s.Attributes.Bg
			}
			if c.Fg == 0 {
				c.Fg = s.Attributes.Fg
			}
			writer.SetCell(term.Coordinates{X: xi, Y: y}, c)
		}
	}
	return
}

func (s *Scroll) wrapdraw(writer fractal.Writer) {
	var xi, yi, ywindow int
	xwindow := s.width
	wraps := 0
	for y, r := range s.buf.RawCells()[s.offset.Y:] {
		ywindow = s.height - wraps
		if y >= ywindow {
			break
		}
		for x, c := range r {
			if c.Ch == 0 {
				continue
			}
			xi = x
			if xi >= xwindow {
				for xi >= xwindow {
					xi -= xwindow
				}
				if xi == 0 {
					wraps++
				}
			}
			yi = y + wraps
			if c.Bg == 0 {
				c.Bg = s.Attributes.Bg
			}
			if c.Fg == 0 {
				c.Fg = s.Attributes.Fg
			}
			writer.SetCell(term.Coordinates{X: xi, Y: yi}, c)
		}
	}

}

// Draw draws the contents of this scroll to the given writer. If Wrap is set,
// lines that are too long wrap around and thus are rendered in the next line.
func (s *Scroll) Draw(writer fractal.Writer) {
	if s.Wrap {
		s.wrapdraw(writer)
		return
	}

	s.draw(writer)
}

// Search performs a text search of text in the internal cell buffer. It populates
// a search list so SeekNextResult and SeekPreviousResult can be used to visualize results.
// It returns the number of matches found. Note that it also sets the attributes of
// the matching terms as defined by ResultsAttr.
func (s *Scroll) Search(text string) (n int) {
	s.searchText = []rune(text)
	return s.searcher.Search(text)
}

// PrevResult returns the coordinates of the previous result in the Search list.
func (s *Scroll) PrevResult() (pos term.Coordinates, ok bool) {
	return s.searcher.PrevResult()
}

// NextResult returns the coordinates of the next result in the Search list.
func (s *Scroll) NextResult() (pos term.Coordinates, ok bool) {
	return s.searcher.NextResult()
}

// Result returns the current search result's coordinates.
func (s *Scroll) Result() (pos term.Coordinates, ok bool) {
	return s.searcher.Result()
}

// Offset returns the scroll offset from the start of the content.
func (s *Scroll) Offset() term.Coordinates {
	return s.offset
}

// Width returns this scroll's width.
func (s *Scroll) Width() int {
	return s.width
}

// Height returns this scroll's height.
func (s *Scroll) Height() int {
	return s.height
}

// Buffer returns s internal Buffer.
func (s *Scroll) Buffer() *cell.Buffer {
	return s.buf
}

// ReadFrom see cell.Buffer.ReadFrom.
func (s *Scroll) ReadFrom(r io.Reader) (n int64, err error) {
	return s.buf.ReadFrom(r)
}
