package component

import (
	"io"
	"strings"

	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
)

// Scroll adds Draw to a Buffer along with
// scrolling, searching and wrap-around capabilities.
type Scroll struct {
	buf           *cell.Buffer
	searcher      cell.SubscriberSearcher
	wrapsLen      int
	wraps         []int
	width, height int
	searchText    []rune
	offset        term.Coordinates
	subs          []ScrollSubscriber

	disablePublishing   bool
	lastPublishedOffset term.Coordinates

	// Sets the search result attributes upon matching.
	ResultsAttr term.Attributes
	Attributes  term.Attributes

	// Debug enables seeing visualizing term cells.
	Debug bool

	// lines longer than the width of the scroll wrap around and
	// are rendered in the next line if Wrap is set to true.
	// Wrap invalidates Debug.
	Wrap bool
}

// NewScroll allocates storage for a Scroll and initializes it.
func NewScroll(buf *cell.Buffer) (s *Scroll) {
	s = new(Scroll)
	s.Init(buf)
	return
}

func (s *Scroll) initBuffer(buf *cell.Buffer) {
	s.buf = buf

	searcher := cell.NewSimpleSearcher(s.buf)
	attrSearcher := cell.AttrSearcher(searcher, s.buf, s.ResultsAttr)

	s.searcher = attrSearcher
	s.buf.Subscribe(s.searcher)
}

// Init initializes this scroll with buf.
func (s *Scroll) Init(buf *cell.Buffer) {
	if s.ResultsAttr == (term.Attributes{}) {
		s.ResultsAttr.Fg, s.ResultsAttr.Bg = term.AttrReverse, term.AttrReverse
	}

	// Searcher that actually performs the text search
	s.initBuffer(buf)

	s.wraps = make([]int, 0)
	s.searchText = nil
	s.offset = term.Coordinates{}
	s.searcher.Reset()
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
	return !s.Wrap && s.offset.X > 0
}

// CanSeekRight returns true if SeekRight would seek one column right.
func (s *Scroll) CanSeekRight() bool {
	return !s.Wrap && s.offset.X < s.getMaxXOffset()
}

// SeekUp shifts the contents of this scroll one row up.
func (s *Scroll) SeekUp() (ok bool) {
	if ok = s.CanSeekUp(); ok {
		dispatch := s.dispatchSubscribers()
		defer dispatch()
		s.offset.Y--
	}
	return ok
}

// SeekDown shifts the contents of this scroll one row down.
func (s *Scroll) SeekDown() (ok bool) {
	if ok = s.CanSeekDown(); ok {
		dispatch := s.dispatchSubscribers()
		defer dispatch()
		s.offset.Y++
	}
	return
}

// SeekLeft shifts the contents of this scroll one column left.
func (s *Scroll) SeekLeft() (ok bool) {
	if ok = s.CanSeekLeft(); ok {
		dispatch := s.dispatchSubscribers()
		defer dispatch()
		s.offset.X--
	}
	return
}

// SeekRight shifts the contents of this scroll one column right.
func (s *Scroll) SeekRight() (ok bool) {
	if ok = s.CanSeekRight(); ok {
		dispatch := s.dispatchSubscribers()
		defer dispatch()
		s.offset.X++
	}
	return
}

// SeekVertical shifts the contents of this scroll such that
// the vertical offset is y.
func (s *Scroll) SeekVertical(y int) (ok bool) {
	return s.seekVertical(y)
}

func (s *Scroll) seekVertical(y int) (ok bool) {
	if y < 0 {
		y = 0
	}

	ok = s.offset.Y != y
	if ok {
		dispatch := s.dispatchSubscribers()
		defer dispatch()
		s.offset.Y = y
	}
	return
}

// SeekHorizontal shifts the contents of this scroll such that
// the horizontal offset is x.
func (s *Scroll) SeekHorizontal(x int) (ok bool) {
	return s.seekHorizontal(x)
}

func (s *Scroll) seekHorizontal(x int) (ok bool) {
	if x < 0 {
		x = 0
	}

	ok = s.offset.X != x
	if ok {
		dispatch := s.dispatchSubscribers()
		defer dispatch()
		s.offset.X = x
	}
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

func (s *Scroll) seekTo(pos term.Coordinates, xpadding, ypadding int) bool {
	var yok, xok bool
	if max := s.getMaxXOffset(); xpadding > max {
		xpadding = max
	}
	if max := s.getMaxYOffset(); ypadding > max {
		ypadding = max
	}

	// ypadding < 0 is used to signal seek on the y axis with no padding
	// setting the exact position to pos.Y, rather than ensuring that pos.Y
	// is within view.
	if ypadding == -1 {
		yok = s.seekVertical(pos.Y)
	} else if pos.Y >= s.offset.Y+s.height-ypadding {
		yok = s.seekVertical(pos.Y - s.height + ypadding)
	} else if pos.Y < s.offset.Y-ypadding {
		yok = s.seekVertical(pos.Y - ypadding)
	}

	if pos.X >= s.offset.X+s.width-xpadding {
		xok = s.seekHorizontal(pos.X - s.width + xpadding)
	} else if pos.X < s.offset.X-xpadding {
		xok = s.seekHorizontal(pos.X - xpadding)
	}

	ok := yok || xok
	return ok
}

// SeekTo shifts the contents of this scroll to make sure that pos is in
// range for the next call to Draw. If position is beyond
// the last seekable content, the max seek position is used as the
// new seek position.
func (s *Scroll) SeekTo(pos term.Coordinates) bool {
	if pos.X < 0 || pos.Y < 0 {
		panic("invalid coordinates: negative")
	}
	if max := s.getMaxXOffset(); pos.X > max {
		pos.X = max
	}
	if max := s.getMaxYOffset(); pos.Y > max {
		pos.Y = max
	}
	return s.seekTo(pos, 1, 1)
}

// SetOffset force-sets the underlying offset of this scroll.
// It is up to the caller to ensure that pos is within the max
// offset. Negative coordinates will trigger a panic.
func (s *Scroll) SetOffset(pos term.Coordinates) bool {
	if pos.X < 0 || pos.Y < 0 {
		panic("invalid coordinates: negative")
	}
	ok := pos != s.offset
	if ok {
		dispatch := s.dispatchSubscribers()
		defer dispatch()
		s.offset = pos
	}
	return ok
}

// SeekNextResult shifts the contents of this scroll to visualize
// the next result in the result list.
func (s *Scroll) SeekNextResult() bool {
	pos, ok := s.NextResult()
	if !ok {
		return false
	}

	return s.seekTo(pos, len(s.searchText), -1)
}

// SeekPrevResult shifts the contents of this scroll to visualize
// the previous result in the result list.
func (s *Scroll) SeekPrevResult() bool {
	pos, ok := s.PrevResult()
	if !ok {
		return false
	}

	return s.seekTo(pos, len(s.searchText), -1)
}

// Resize resizes this scroll to fit inside given width and height.
func (s *Scroll) Resize(width, height int) {
	s.width = width
	s.height = height
	// do not re-calculate offsets here as it should trigger
	// dispatching subscribers OnWillSeek/OnDidSeek, but we
	// shouldn't do that on calls to Resize.
}

func (s *Scroll) getMaxXOffset() (x int) {
	if s.Wrap {
		return
	}

	columns := s.buf.MaxColumns()
	if columns >= s.width {
		x = columns - s.width + 1
	}
	return
}

// RowsWithWraps returns the number of rows in this scroll,
// including the chunks of lines that were wrapped around.
func (s *Scroll) RowsWithWraps() int {
	return s.buf.Rows() + s.wrapsLen
}

func (s *Scroll) getMaxYOffset() (y int) {
	rows := s.RowsWithWraps()
	if rows >= s.height {
		y = rows - s.height
	}
	return
}

func (s *Scroll) rawCellsOffsetNoWrap() [][]term.Cell {
	cells := s.buf.RawCells()
	if s.offset.Y <= len(cells) {
		return cells[s.offset.Y:]
	}
	// this can happen in some cases when content is modified
	// outside scroll and offset.Y is simply stale.
	if len(cells) > 0 {
		return cells[len(cells)-1:]
	}
	return cells[:]
}

func (s *Scroll) drawNoAttr(writer term.Writer) {
	xwindow := s.offset.X + s.width
	ywindow := s.height
	for y, r := range s.rawCellsOffsetNoWrap() {
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
			writer.SetCell(term.Coordinates{X: xi, Y: y}, c)
		}
	}
	return
}

func (s *Scroll) drawDebug(writer term.Writer) {
	xwindow := s.offset.X + s.width
	ywindow := s.height
	for y, r := range s.rawCellsOffsetNoWrap() {
		if y >= ywindow {
			break
		}
		var x int
		var c term.Cell
		for x, c = range r {
			if x >= xwindow {
				break
			}
			if c.Ch == 0 {
				c.Ch = '░'
			}
			if x < s.offset.X {
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
		if x >= xwindow {
			break
		}
		x = x - s.offset.X
		if len(r) != 0 {
			x++
		}
		writer.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{Ch: '¬'})
	}
	return
}

func (s *Scroll) draw(writer term.Writer) {
	xwindow := s.offset.X + s.width
	ywindow := s.height
	for y, r := range s.rawCellsOffsetNoWrap() {
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

func (s *Scroll) wrapdrawNoAttr(writer term.Writer) {
	s.wraps, s.wrapsLen = doWrapdrawNoAttr(writer, s.buf, s.width, s.height, s.offset)
}

func doWrapdrawNoAttr(
	writer term.Writer, buf *cell.Buffer,
	width, height int, offset term.Coordinates,
) (wraps []int, wrapsLen int) {
	xwindow := width
	ywindow := height
	wraps = make([]int, 0)
	wrapsLen = 0
	for y, r := range buf.RawCells() {
		var count int
		// cannot skip any row until all wraps are accounted for
		for x, c := range r {
			xi := x
			if xi >= xwindow {
				for xi >= xwindow {
					xi -= xwindow
				}
				if xi == 0 {
					count++
					wrapsLen++
				}
			}
			yi := y + wrapsLen - offset.Y
			if yi >= 0 && yi < ywindow && c.Ch != 0 {
				writer.SetCell(term.Coordinates{X: xi, Y: yi}, c)
			}
		}
		wraps = append(wraps, count)
	}
	return
}

func (s *Scroll) wrapdraw(writer term.Writer) {
	xwindow := s.width
	ywindow := s.height
	s.wraps = make([]int, 0)
	s.wrapsLen = 0
	for y, r := range s.buf.RawCells() {
		var count int
		// cannot skip any row until wraps are accounted for
		for x, c := range r {
			xi := x
			if xi >= xwindow {
				for xi >= xwindow {
					xi -= xwindow
				}
				if xi == 0 {
					count++
					s.wrapsLen++
				}
			}
			yi := y + s.wrapsLen - s.offset.Y
			if yi >= 0 && yi < ywindow && c.Ch != 0 {
				if c.Bg == 0 {
					c.Bg = s.Attributes.Bg
				}
				if c.Fg == 0 {
					c.Fg = s.Attributes.Fg
				}
				writer.SetCell(term.Coordinates{X: xi, Y: yi}, c)
			}
		}
		s.wraps = append(s.wraps, count)
	}
}

// RecalculateWraps can be used to signal Scroll that buffer has been updated
// and so calculated wrap properties might be incorrect.
func (s *Scroll) RecalculateWraps() {
	if s.Wrap {
		s.Draw(term.NoopWriter{})
	}
}

// Draw draws the contents of this scroll to the given writer. If Wrap is set,
// lines that are too long wrap around and thus are rendered in the next line.
func (s *Scroll) Draw(writer term.Writer) {
	if s.width <= 0 || s.height <= 0 {
		return
	}
	if s.Attributes == (term.Attributes{}) {
		if s.Wrap {
			s.wrapdrawNoAttr(writer)
			return
		}

		if s.Debug {
			s.drawDebug(writer)
			return
		}

		s.drawNoAttr(writer)
		return
	}

	// setcell background
	for y := 0; y < s.height; y++ {
		for x := 0; x < s.width; x++ {
			writer.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
				Fg: s.Attributes.Fg,
				Bg: s.Attributes.Bg,
			})
		}
	}

	if s.Wrap {
		s.wrapdraw(writer)
		return
	}

	if s.Debug {
		s.drawDebug(writer)
		return
	}

	s.draw(writer)
}

// WordAt returns the word at the given position or an empty string if token
// at the given position is not a word. See TokenAt for more details.
func (s *Scroll) WordAt(pos term.Coordinates) (
	term.Coordinates, term.Coordinates, string,
) {
	return s.TokenAt(pos, func(c rune) bool {
		return (c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') || c == '_' ||
			(c >= '0' && c <= '9')
	})
}

// TokenAt returns the token that satisfies the isAllowed function
// and its start and end positions.
//
// It returns an empty string if the token at the given position does
// not satisfy isAllowed.
func (s *Scroll) TokenAt(pos term.Coordinates, isAllowed func(rune) bool) (
	term.Coordinates, term.Coordinates, string,
) {
	var b strings.Builder
	cells := s.buf.RawCells()
	rows := s.buf.Rows()

	start := pos
	for start.Y < rows && start.X < s.buf.Columns(start.Y) && start.X >= 0 {
		c := cells[start.Y][start.X]
		if isAllowed(c.Ch) {
			start.X--
			continue
		}
		break
	}

	if start == pos {
		return start, start, b.String()
	}

	start.X++
	end := start
	for end.Y < rows && end.X < s.buf.Columns(end.Y) && end.X >= 0 {
		c := cells[end.Y][end.X]
		if isAllowed(c.Ch) {
			end.X++
			b.WriteRune(c.Ch)
			continue
		}
		break
	}

	return start, end, b.String()
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

// SizeHeight returns this scroll's height as prescribed
// by the last call to Resize. It's named
// SizeHeight to differentiate from Height, which is used
// to satisfy component.Responsive.
func (s *Scroll) SizeHeight() int {
	return s.height
}

// Height satisfies component.Responsive.
func (s *Scroll) Height(width int) int {
	rows := s.buf.Rows()
	if !s.Wrap {
		return rows
	}
	if width == 0 {
		return 0
	}
	// NOTE: Height shouldn't rely on any internal mutable state
	// except for the buffer.
	_, wrapsLen := doWrapdrawNoAttr(term.NoopWriter{}, s.buf,
		width, 0, term.Coordinates{})
	return rows + wrapsLen
}

// Wraps returns all the wraps detected in the last call to Draw.
//
// If Draw has not been called yet, then this method returns an
// empty map.
func (s *Scroll) Wraps() []int {
	return s.wraps
}

// Buffer returns s internal Buffer.
func (s *Scroll) Buffer() *cell.Buffer {
	return s.buf
}

// ReadFrom see cell.Buffer.ReadFrom.
func (s *Scroll) ReadFrom(r io.Reader) (n int64, err error) {
	return s.buf.ReadFrom(r)
}

// ScrollSubscriber is a subscriber of seek operations in a scroll.
type ScrollSubscriber interface {
	// OnWillSeek is dispatched before a scroll is about to change its offset.
	OnWillSeek(from term.Coordinates)
	// OnDidSeek is dispatched after a scroll has changed its offset.
	OnDidSeek(from, to term.Coordinates)
}

type fnSubscriber func(term.Coordinates, term.Coordinates)

func (s fnSubscriber) OnWillSeek(from term.Coordinates) {
	/* no-op */
}

func (s fnSubscriber) OnDidSeek(from, to term.Coordinates) {
	s(from, to)
}

func (s *Scroll) dispatchSubscribers() func() {
	if s.disablePublishing {
		return func() {}
	}
	from := s.offset
	for _, sub := range s.subs {
		sub.OnWillSeek(from)
	}
	return func() {
		if s.disablePublishing {
			return
		}

		enable := s.DisablePublishing()
		defer enable()

		to := s.offset
		for _, sub := range s.subs {
			sub.OnDidSeek(from, to)
		}
		s.lastPublishedOffset = to
	}
}

// Subscribe subscribes sub to seek operations.
func (s *Scroll) Subscribe(sub ScrollSubscriber) {
	s.subs = append(s.subs, sub)
}

// DisablePublishing disables dispatching OnDidSeek/OnWillSeek calls to subscribers.
// This is useful when clients of Scroll perform composite moves that
// would otherwise dispatch multiple calls rather than one.
// The returned function can be called to re-enable publishing.
//
// This method can be called multiple times and only the first time
// will disable, and only the first returned enable will re-enable
// publishing.
//
// The returned function, dispatches an OnWillSeek/OnDidSeek call pair to
// each subscriber if the offset has changed since last time publishing was disabled.
// Note that OnWillSeek in this case will be dispatched after the offset is changed
// so callers must ensure that any state that needs capturing is captured
// before using DisablePublishing/EnablePublishing.
func (s *Scroll) DisablePublishing() (enable func()) {
	if s.disablePublishing {
		return func() {}
	}
	s.disablePublishing = true
	return s.enablePublishing
}

// enablePublishing enables dispatching OnWillSeek/OnDidSeek calls
// after a call to DisablePublishing. It dispatches an OnWillSeek/OnDidSeek call pair to
// each subscriber if the offset has changed since last time publishing was disabled.
// Note that OnWillSeek in this case will be dispatched after the offset is changed
// so callers must ensure that any state that needs capturing is captured
// before using DisablePublishing/EnablePublishing.
func (s *Scroll) enablePublishing() {
	s.disablePublishing = false
	if s.lastPublishedOffset != s.Offset() {
		dispatch := s.dispatchSubscribers()
		dispatch()
	}
}

// PublishingEnabled returns whether publishing has been
// enabled with EnablePublishing, or disabled with DisablePublishing. By default
// it is enabled when Scroll is initialized.
func (s *Scroll) PublishingEnabled() bool {
	return !s.disablePublishing
}

// FuncScrollSubscriber wraps fn to satisfy ScrollSubscriber.
func FuncScrollSubscriber(fn func(from, to term.Coordinates)) ScrollSubscriber {
	return fnSubscriber(fn)
}
