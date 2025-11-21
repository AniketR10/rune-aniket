// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package component

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"

	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
)

// Scroll adds Draw to a Buffer along with
// scrolling, searching and wrap-around capabilities.
type Scroll struct {
	buf           *cell.Buffer
	searcher      cell.Searcher
	matches       []term.Coordinates
	wrapsLen      int
	wraps         []int
	width, height int
	searchText    []rune
	searchTextStr string
	offset        term.Coordinates
	subs          []ScrollSubscriber
	hiddenblocks  map[int]int
	hiddenlines   map[int]int
	hiddenmeta    map[int]string
	hiddensorted  []startEndBlock

	onWillEditFrom term.Coordinates
	onWillEditTo   term.Coordinates

	disablePublishing   bool
	lastPublishedOffset term.Coordinates

	// ResultsAttr determines the search result attributes upon matching.
	ResultsAttr term.Attributes

	// Attributes determines the attributes for regular cells.
	Attributes term.Attributes

	// Debug enables seeing visualizing term cells.
	Debug bool

	// rows longer than the width of the scroll wrap around and
	// are rendered in the next row if Wrap is set to true.
	// Wrap invalidates Debug.
	Wrap bool

	// HideAttr determines the attributes used to mark hidden rows.
	HideAttr term.Attributes
}

var _ Scrollable = (*Scroll)(nil)

// NewScroll allocates storage for a Scroll and initializes it.
func NewScroll(buf *cell.Buffer) (s *Scroll) {
	s = new(Scroll)
	s.Init(buf)
	return
}

func (s *Scroll) initBuffer(buf *cell.Buffer) {
	s.buf = buf
	s.searcher = cell.NewSimpleSearcher(s.buf)
	s.searcher.Reset()
}

// Init initializes this scroll with buf.
func (s *Scroll) Init(buf *cell.Buffer) {
	s.InitPerformance(buf)
	s.buf.Subscribe((*scrollSubscriber)(s))
	s.hiddenblocks = make(map[int]int)
	s.hiddenlines = make(map[int]int)
	s.hiddenmeta = make(map[int]string)
	if s.HideAttr == (term.Attributes{}) {
		s.HideAttr = s.Attributes
		s.HideAttr.Fg = tcell.ColorGray
	}
}

// InitPerformance initializes this scroll with limited search functionality:
// Updates to buffer do not update search results. Clearing of search results
// should be done manually by clients by calling Search again after updates,
// if desired.
//
// Note that this initialization method should be used instead of Init
// if the given cell.Buffer has been also initialized with InitPerformance.
//
// MarkHidden is disabled.
func (s *Scroll) InitPerformance(buf *cell.Buffer) {
	if s.ResultsAttr == (term.Attributes{}) {
		s.ResultsAttr.Attrs = tcell.AttrReverse
	}
	s.initBuffer(buf)
	s.wraps = make([]int, 0)
}

// CanSeekUp returns true if SeekUp would seek one row up.
func (s *Scroll) CanSeekUp() bool {
	return s.offset.Y > 0
}

// CanSeekDown returns true if SeekDown would seek one row down.
func (s *Scroll) CanSeekDown() bool {
	return s.offset.Y < s.getMaxYOffset()
}

// MaxSeekOffset returns the max seek offset.
func (s *Scroll) MaxSeekOffset() int {
	return s.getMaxYOffset()
}

// SeekOffset returns the current seek offset.
func (s *Scroll) SeekOffset() int {
	return s.offset.Y
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

// HiddenLineCount returns the total number of hidden lines.
func (s *Scroll) HiddenLineCount() int {
	return len(s.hiddenlines)
}

// MarkHidden marks the inner rows of the given row range as hidden,
// so next call to Draw will not display them, and instead display an icon to indicate
// that there are hidden rows.
func (s *Scroll) MarkHidden(start, end int) bool {
	if start > end {
		tmp := end
		end = start
		start = tmp
	}
	if start == end {
		return false
	}
	if start < 0 || end >= s.buf.Rows() || s.hiddenblocks == nil {
		return false
	}

	// do not process if new fold is inside a current fold
	for bstart, bend := range s.hiddenblocks {
		if start >= bstart && start <= bend && end >= bstart && end <= bend {
			return false
		}
	}

	// remove any folds that intersect with the new fold:
	// this avoids problems calculating coordinates
	for bstart, bend := range s.hiddenblocks {
		if (bstart >= start && bstart <= end) || (bend <= end && bend >= start) {
			delete(s.hiddenblocks, bstart)
			s.dispatchVisibleToSubscribers(bstart)
		}
	}

	// update offset subscribers when there's a change in hidden lines
	s.hiddenblocks[start] = end
	s.rebuildHiddenLines()

	s.dispatchHiddenToSubscribers(start, end)
	return true
}

// HiddenBlockAt returns the hidden block starting at line, or false
// if there's no hidden block.
func (s *Scroll) HiddenBlockAt(line int) (ret term.Range, ok bool) {
	if s.hiddenblocks == nil {
		return
	}
	end, ok := s.hiddenblocks[line]
	if !ok {
		return
	}
	ret = term.Range{
		Start: term.Coordinates{Y: line},
		End:   term.Coordinates{Y: end},
	}
	return ret, ok
}

// MarkVisible reverses MarkHidden for the given row block start.
func (s *Scroll) MarkVisible(start int) bool {
	if s.hiddenblocks == nil {
		return false
	}
	_, ok := s.hiddenblocks[start]
	if ok {
		delete(s.hiddenblocks, start)
		s.rebuildHiddenLines()
		s.dispatchVisibleToSubscribers(start)
	}
	return ok
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
		x = columns - s.width
	}
	return
}

// RowsWithWraps returns the number of rows in this scroll,
// including the chunks of rows that were wrapped around.
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

func (s *Scroll) rawCellsOffsetNoWrap(hiddenOffset int) [][]term.Cell {
	cells := s.buf.RawCells()
	offset := s.offset.Y + hiddenOffset
	if offset <= len(cells) {
		return cells[offset:]
	}
	// this can happen in some cases when content is modified
	// outside scroll and offset.Y is simply stale.
	if len(cells) > 0 {
		return cells[len(cells)-1:]
	}
	return cells[:]
}

func (s *Scroll) hiddenOffset() (ret int) {
	for _, t := range s.hiddensorted {
		if t.start < s.offset.Y+ret {
			ret += t.end - t.start
		}
	}
	return
}

type startEndBlock struct {
	start int
	end   int
}

func (s *Scroll) drawNoAttr(writer term.Writer) {
	xwindow := s.offset.X + s.width
	ywindow := s.height
	for y, r := range s.rawCellsOffsetNoWrap(0) {
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
}

func (s *Scroll) draw(writer term.Writer) {
	xwindow := s.offset.X + s.width
	ywindow := s.height
	for y, r := range s.rawCellsOffsetNoWrap(0) {
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
}

func (s *Scroll) drawDebug(writer term.Writer) {
	xwindow := s.offset.X + s.width
	ywindow := s.height
	for y, r := range s.rawCellsOffsetNoWrap(0) {
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
// rows that are too long wrap around and thus are rendered in the next row.
func (s *Scroll) Draw(writer term.Writer) {
	if s.width <= 0 || s.height <= 0 {
		return
	}

	defer s.drawSearchResults(writer)

	if s.Attributes == (term.Attributes{}) {
		if s.Wrap {
			s.wrapdrawNoAttr(writer)
			return
		}

		if s.Debug {
			s.drawDebug(writer)
			return
		}

		if len(s.hiddenblocks) != 0 {
			s.drawWithHidden(writer)
			return
		}

		s.drawNoAttr(writer)
		return
	}

	// setcell background
	for y := 0; y < s.height; y++ {
		for x := 0; x < s.width; x++ {
			writer.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
				Attributes: s.Attributes,
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

	if len(s.hiddenblocks) != 0 {
		s.drawWithHidden(writer)
		return
	}

	s.draw(writer)
}

// WordAt returns the word at the given position or an empty string if token
// at the given position is not a word. See TokenAt for more details.
func (s *Scroll) WordAt(pos term.Coordinates) (
	term.Coordinates, term.Coordinates, string,
) {
	return s.TokenAt(pos, wordMatcher)
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
	s.searchTextStr = text
	return s.search()
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

	// OnHide is dispatched when a scroll has hidden a block of lines.
	OnHide(start, end int)
	// OnVisible is dispatched when a scroll has made visible a block of lines,
	// that was previously hidden.
	OnVisible(start int)
}

type fnSubscriber func(term.Coordinates, term.Coordinates)

func (s fnSubscriber) OnWillSeek(from term.Coordinates) {
	/* no-op */
}

func (s fnSubscriber) OnHide(start, end int) {
	/* no-op */
}

func (s fnSubscriber) OnVisible(start int) {
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

func (s *Scroll) dispatchVisibleToSubscribers(start int) {
	if s.disablePublishing {
		return
	}
	for _, sub := range s.subs {
		sub.OnVisible(start)
	}
}

func (s *Scroll) dispatchHiddenToSubscribers(start, end int) {
	if s.disablePublishing {
		return
	}
	for _, sub := range s.subs {
		sub.OnHide(start, end)
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

// FuncScrollSubscriber wraps fn to satisfy ScrollSubscriber, which is called
// every time the offset changes. Changes to line visibility will be ignored.
func FuncScrollSubscriber(fn func(from, to term.Coordinates)) ScrollSubscriber {
	return fnSubscriber(fn)
}

// ScrollToWindowCoordinates translates scroll content Coordinates to window Coordinates,
// taking into consideration scroll offsets and wrapped rows. The second boolean return
// value is used to indicate that the given position is inside a hidden block (false), or
// not hidden (true). A valid set of coordinates is returned in either case, but when the
// coordinates would fall inside a hidden block, the start of the hidden block is returned.
func (s *Scroll) ScrollToWindowCoordinates(pos term.Coordinates) (term.Coordinates, bool) {
	offset := s.Offset()
	ret := term.CoordinatesDiff(pos, offset)
	if !s.Wrap {
		if len(s.hiddensorted) != 0 {
			for _, block := range s.hiddensorted {
				if block.start > pos.Y {
					break
				}
				if pos.Y > block.start {
					// min in case pos is inside block
					ret.Y -= min(block.end, pos.Y) - block.start
					if pos.Y <= block.end {
						return ret, false
					}
				}
			}
		}
		return ret, true
	}
	wraps := s.Wraps()
	for y, count := range wraps {
		if y >= pos.Y {
			break
		}
		ret.Y += count
	}
	diff := pos.X / s.Width()
	ret.X = pos.X%s.Width() - offset.X
	ret.Y += diff
	return ret, true
}

// WindowToScrollCoordinates translates window Coordinates to scroll content Coordinates,
// taking into consideration scroll offsets and wrapped rows.
func (s *Scroll) WindowToScrollCoordinates(pos term.Coordinates) term.Coordinates {
	offset := s.Offset()
	ret := term.CoordinatesSum(pos, offset)
	if !s.Wrap {
		if len(s.hiddensorted) != 0 {
			for _, block := range s.hiddensorted {
				if block.start > ret.Y {
					break
				}
				if ret.Y > block.start {
					ret.Y += block.end - block.start
				}
			}
		}
		return ret
	}
	wraps := s.Wraps()
	var sum, count int
	for _, count = range wraps {
		if pos.Y+offset.Y >= sum+count+1 {
			ret.Y -= count
			sum += count + 1
			continue
		}
		diff := pos.Y + offset.Y - sum
		ret.X = pos.X + diff*s.Width() + offset.X
		ret.Y -= diff
		break
	}
	return ret
}

func wordMatcher(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func (s *Scroll) drawSearchResults(w term.Writer) {
	slen := len(s.searchText)
	for _, posAtScroll := range s.matches {
		posAtScreen, ok := s.ScrollToWindowCoordinates(posAtScroll)
		if !ok || posAtScreen.Y >= s.height || posAtScreen.Y < 0 {
			continue
		}

		toX := posAtScroll.X + slen
		for x := posAtScroll.X; x < toX; x++ {
			posAtScreen, ok := s.ScrollToWindowCoordinates(term.Coordinates{Y: posAtScroll.Y, X: x})
			if !ok || posAtScreen.X >= s.width || posAtScreen.X < 0 {
				continue
			}
			w.UnionAttributes(posAtScreen, s.ResultsAttr)
		}
	}
}

func (s *Scroll) search() (n int) {
	s.matches = s.matches[:0]

	n = s.searcher.Search(s.searchTextStr)
	for i := 0; i < n; i++ {
		pos, ok := s.searcher.NextResult()
		if !ok {
			panic("searcher return n results but no enough results available")
		}
		s.matches = append(s.matches, pos)
	}
	return
}

type scrollSubscriber Scroll

func (s *scrollSubscriber) OnWillEdit(
	ctx context.Context, from, to term.Coordinates, str string,
) {
	s.onWillEditFrom = from
	s.onWillEditTo = to
}

func (s *scrollSubscriber) OnDidEdit(
	ctx context.Context, start, end term.Coordinates, str string,
) {
	// if any match doesn't match, then re-issue search
	for _, match := range s.matches {
		for i := 0; i < len(s.searchText); i++ {
			cc, ok := s.buf.Cell(match)
			if !ok || cc.Ch != s.searchText[i] {
				(*Scroll)(s).search()
				return
			}
			match.X++
		}
	}

	clear(s.hiddenblocks)
	// if lines above hidden lines were either added or removed
	// (or both) then update hidden locations
	removed := s.onWillEditTo.Y - s.onWillEditFrom.Y
	added := end.Y - start.Y
	for _, hidden := range s.hiddensorted {
		// clear blocks that are partially deleted
		if (s.onWillEditFrom.Y <= hidden.end && s.onWillEditFrom.Y >= hidden.start) ||
			(s.onWillEditTo.Y >= hidden.start && s.onWillEditTo.Y <= hidden.end) {
			continue
		}
		var delta int
		if s.onWillEditFrom.Y < hidden.start && s.onWillEditTo.Y < hidden.start {
			delta -= removed
		}
		if start.Y < hidden.start {
			delta += added
		}
		s.hiddenblocks[hidden.start+delta] = hidden.end + delta
	}

	(*Scroll)(s).rebuildHiddenLines()
}

func (s *Scroll) drawWithHidden(writer term.Writer) {
	xwindow := s.offset.X + s.width
	ywindow := s.height
	var targety, hideLineIconOffset int
	endblock := -1
	hiddenOffset := s.hiddenOffset()
	cells := s.rawCellsOffsetNoWrap(hiddenOffset)
	for y := 0; y < len(cells); y++ {
		r := cells[y]
		if targety >= ywindow {
			break
		}
		scrollY := y + s.offset.Y + hiddenOffset
		if scrollY < endblock {
			continue
		}
		if scrollY == endblock {
			trim := true
			targetx := hideLineIconOffset
			for _, c := range r {
				if targetx >= s.width {
					break
				}
				if c.Ch == 0 || targetx < 0 || (trim && (c.Ch == ' ' || c.Ch == '\t')) {
					continue
				}
				if c.Bg == 0 {
					c.Bg = s.Attributes.Bg
				}
				if c.Fg == 0 {
					c.Fg = s.Attributes.Fg
				}
				trim = false
				writer.SetCell(term.Coordinates{X: targetx, Y: targety}, c)
				targetx++
			}
			targety++
			continue
		}
		// the first hidden line in hiddenlines
		end, ok := s.hiddenlines[scrollY]
		_, isBlockStart := s.hiddenblocks[scrollY]
		// we're partially scrolled over a block
		if ok && !isBlockStart {
			y = end + s.offset.Y + 1 + hiddenOffset
			continue
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
			writer.SetCell(term.Coordinates{X: xi, Y: targety}, c)
		}
		if ok {
			endblock = end
			// save allocations during Draw by pre-computing this, which doesn't change
			// unless hidden blocks are altered.
			hideLineStr := s.hiddenmeta[scrollY]
			hideLineIconOffset = s.Buffer().Columns(scrollY) - s.offset.X - 1
			if hideLineIconOffset >= s.width {
				continue
			}
			hideLineIconOffset++
			for _, r := range hideLineStr {
				if hideLineIconOffset < 0 {
					hideLineIconOffset++
					continue
				}
				if hideLineIconOffset == s.width {
					break
				}
				cell := term.Cell{Width: 1, Ch: r, Attributes: s.HideAttr}
				writer.SetCell(term.Coordinates{X: hideLineIconOffset, Y: targety}, cell)
				hideLineIconOffset++
			}
			continue
		}
		targety++
	}
}

func (s *Scroll) rebuildHiddenLines() {
	clear(s.hiddenlines)
	for start, end := range s.hiddenblocks {
		for i := start; i <= end; i++ {
			s.hiddenlines[i] = end
		}
	}
	s.hiddensorted = s.hiddensorted[:0]
	var i int
	for start, end := range s.hiddenblocks {
		s.hiddensorted = append(s.hiddensorted, startEndBlock{start, end})
		i++
		s.hiddenmeta[start] = fmt.Sprintf(" [%d lines] ", end-start+1)
	}
	sort.Slice(s.hiddensorted, func(i, j int) bool {
		return s.hiddensorted[i].start < s.hiddensorted[j].start
	})
}
