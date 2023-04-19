package search

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"sync"
	"time"

	fzf "github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

// internal representation of ListConfig
type listConfig struct {
	matchedTextAttr   term.Attributes
	matchCountAttr    term.Attributes
	searchBaseAttr    term.Attributes
	textAttr          term.Attributes
	focusAttr         term.Attributes
	algo              fzf.Algo
	interrupter       term.Interrupter
	caseSensitive     bool
	bottomSearchBar   bool
	interruptEvery    time.Duration
	setFileCountEvery int
}

type matchCounter struct {
	width int
	cell.Buffer
	component.Scroll
	component.Virtual
}

// List is a collection of elements that can be interactively searched.
type List struct {
	mu           sync.Mutex
	quitChan     chan struct{}
	input        [][]byte
	searchCtx    context.Context
	cancelSearch func()
	height       int
	width        int

	cfg listConfig

	searchBar struct {
		component.Responsive
		component.Virtual
		minInputHeight int

		syncBuffer *cell.Buffer
		// this cannot be modified from this component
		// or else syncBuffer becomes out of sync.
		// All updates to the search buffer must
		// be performed via Buffer()
		internalRead *cell.Buffer
		dirty        bool
	}

	matchCountBar matchCounter
	list          struct {
		component.Virtual
		component.FocusList
	}
}

type searchResultComponent struct {
	*component.LazyBytes
	Match
}

// NewList allocates storage for a new List and initializes it.
// See Init for more details.
func NewList(cfg ListConfig) *List {
	l := new(List)
	l.Init(cfg)
	return l
}

// Init initializes this config with cfg. Close must be called
// when this List is no longer to be used, or before Init
// is to be called again to reset the list.
func (l *List) Init(cfg ListConfig) {
	l.cfg = cfg.toInternal()

	l.searchBar.internalRead = cell.NewBuffer()
	l.searchBar.Responsive = component.Buffer(
		l.searchBar.internalRead, component.StringConfig{
			Attributes:           l.cfg.textAttr,
			BackgroundAttributes: l.cfg.textAttr,
		})
	l.searchBar.C = l.searchBar.Responsive
	l.searchBar.dirty = true
	l.searchBar.syncBuffer = cell.NewBuffer()
	l.searchBar.minInputHeight = 1
	l.searchBar.syncBuffer.Subscribe(syncBuffer{
		parent: l,
		buf:    l.searchBar.internalRead,
	})

	l.matchCountBar.init()

	l.list.FocusList.InitWithAttr(l.cfg.textAttr, l.cfg.focusAttr)
	l.list.FocusList.Inverted = l.cfg.bottomSearchBar
	l.list.C = &l.list.FocusList

	l.setFilesCount()
	l.quitChan = make(chan struct{})
}

// ToggleCaseSensitivity toggles whether the search should be case sensitive or not.
// It returns the previous setting and launches a new search asynchronously.
func (l *List) ToggleCaseSensitivity() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	ret := l.cfg.caseSensitive
	l.cfg.caseSensitive = !l.cfg.caseSensitive
	l.asyncSearch()
	return ret
}

func (b *matchCounter) init() {
	b.Buffer.Init()
	b.Scroll.Init(&b.Buffer)
	b.C = &b.Scroll
}

type syncBuffer struct {
	parent *List
	buf    *cell.Buffer
}

func (s syncBuffer) OnWillEdit(start, end term.Coordinates, str string) {
	s.parent.mu.Lock()
	defer s.parent.mu.Unlock()
	s.buf.Edit(start, end, str)
	s.parent.searchBar.dirty = true
	s.parent.asyncSearch()
}

func (s syncBuffer) OnDidEdit(from, to term.Coordinates, old string) {
}

func addMatch(
	list *component.FocusList, match Match,
	textAttr, matchTextAttr term.Attributes,
) {
	// this is a very hot path, performance critical
	// when loading large amounts of data into a search list.
	// Use LazyBytes, which defers all allocations until the next
	// call to Draw, this way all components that do not need to
	// be drawn barely imply any allocations (list uses a *Virtual
	// under the hood but that's about it).
	b := component.LazyBytes{Data: match.data, Attributes: textAttr}
	if match.tokens != nil {
		b.Tokens = *match.tokens
		b.TokenAttributes = matchTextAttr
	}
	list.PushBack(searchResultComponent{
		LazyBytes: &b,
		Match:     match,
	})
}

func sortByResultScore(a, b component.WithAttributes) bool {
	ab := a.(searchResultComponent)
	bb := b.(searchResultComponent)
	if ab.Match.res.Score == bb.Match.res.Score {
		return ab.Match.idx < bb.Match.idx
	}
	return ab.Match.res.Score > bb.Match.res.Score
}

func sortByResultScoreInverted(a, b component.WithAttributes) bool {
	ab := a.(searchResultComponent)
	bb := b.(searchResultComponent)
	if ab.Match.res.Score == bb.Match.res.Score {
		return ab.Match.idx < bb.Match.idx
	}
	return ab.Match.res.Score < bb.Match.res.Score
}

func (l *List) getSearchQuery() string {
	return l.searchBar.internalRead.String()
}

// MatchCount returns how many elements match the search query so far.
func (l *List) MatchCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.list.Len()
}

// TotalCount returns the total number of elements in the list, whether
// they are matches or not.
func (l *List) TotalCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return len(l.input)
}

func doSetFilesCount(matchCountBar *matchCounter, matches, total int, attr term.Attributes) {
	matchCountBar.Reset()
	matchCountBar.WriteStringWithAttr(strconv.Itoa(matches), attr)
	matchCountBar.WriteStringWithAttr("/", attr)
	matchCountBar.WriteStringWithAttr(strconv.Itoa(total), attr)
}

func (l *List) setFilesCount() {
	doSetFilesCount(&l.matchCountBar, l.list.Len(), len(l.input), l.cfg.matchCountAttr)
}

func (l *List) sortMatchesList() {
	if l.cfg.bottomSearchBar {
		l.list.Sort(sortByResultScoreInverted)
	} else {
		l.list.Sort(sortByResultScore)
	}
	l.setFilesCount()
}

func (l *List) pushData(data []byte, slab *util.Slab, sortList bool) (matched bool) {
	linebuf := [1][]byte{nil}
	linebuf[0] = data

	l.mu.Lock()
	defer l.mu.Unlock()

	l.input = append(l.input, data)

	searchInput := l.getSearchQuery()
	if len(searchInput) == 0 {
		m := Match{data: data, idx: len(l.input) - 1}
		matched = true
		addMatch(&l.list.FocusList, m, l.cfg.textAttr, l.cfg.matchedTextAttr)
		if !sortList {
			return
		}
		if l.cfg.bottomSearchBar {
			l.sortMatchesList()
		} else {
			l.setFilesCount()
		}
		return
	}

	search(l.cfg.algo, linebuf[:], searchInput, slab, l.cfg.caseSensitive,
		func(match Match) bool {
			match.idx = len(l.input) - 1
			matched = true
			addMatch(&l.list.FocusList, match, l.cfg.textAttr, l.cfg.matchedTextAttr)
			return false
		})

	if sortList {
		l.sortMatchesList()
	}

	return
}

func (l *List) consumeAsyncElements(ctx context.Context, datachan chan []byte, quitChan chan struct{}) {
	t := time.NewTicker(l.cfg.interruptEvery)
	tickerCh := make(chan struct{}) // need a way to signal from below
	defer close(tickerCh)
	defer func() {
		l.mu.Lock()
		if len(l.getSearchQuery()) == 0 {
			l.setFilesCount()
		} else {
			l.sortMatchesList()
		}
		interrupter := l.cfg.interrupter
		l.mu.Unlock()
		interrupter.Interrupt()
	}()

	var dirty bool
	go func() {
		defer t.Stop()
		for {
			select {
			case <-t.C:
				l.mu.Lock()
				if !dirty {
					l.mu.Unlock()
					continue
				}
				interrupter := l.cfg.interrupter
				if len(l.getSearchQuery()) != 0 || l.cfg.bottomSearchBar {
					l.sortMatchesList()
				} else {
					l.setFilesCount()
				}
				dirty = false
				l.mu.Unlock()
				interrupter.Interrupt()
			case <-ctx.Done():
				return
			case <-quitChan:
				return
			case <-tickerCh:
				return
			}
		}
	}()

	slab := makeSlab()
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-datachan:
			if !ok {
				return
			}
			l.pushData(data, slab, false)
			if i%l.cfg.setFileCountEvery == 0 {
				l.mu.Lock()
				l.setFilesCount()
				dirty = true
				l.mu.Unlock()
			}
		case <-quitChan:
			l.DataReset()
			return
		}
	}
}

func (l *List) handleSearch(
	ctx context.Context, cancelFn func(), input [][]byte,
	searchInput string,
) {
	slab := makeSlab()
	search(l.cfg.algo, input, searchInput, slab, l.cfg.caseSensitive,
		func(match Match) bool {
			// NOTE: this creates a lot of contention when performing queries
			// on very large inputs that are still being collected via Push.
			// search list should be refactor to use on goroutine which takes
			// requests of either: new search (with query + all input), new data, or draw
			// that should be the only goroutine with access to l.list
			l.mu.Lock()
			defer l.mu.Unlock()
			select {
			case <-ctx.Done():
				return false
			default:
			}
			addMatch(&l.list.FocusList, match, l.cfg.textAttr, l.cfg.matchedTextAttr)
			return true
		})

	l.mu.Lock()
	l.sortMatchesList()
	cancelFn()
	interrupter := l.cfg.interrupter
	l.mu.Unlock()
	interrupter.Interrupt()
}

// Push returns a channel that can be used to push data to this list asynchronously.
// Clients should call close on the channel if no more data is expected.
// See PushSync for more details.
func (l *List) Push(ctx context.Context) chan<- []byte {
	datachan := make(chan []byte)
	go l.consumeAsyncElements(ctx, datachan, l.quitChan)
	return datachan
}

// Pause hints to this list that no more data is expected, for now.
// This should be called when pushing an initial large amount of data
// from an unbound source.
//
// Note that this should NOT be called when there's no more data
// remaining, in which case closing the channel returned by Push is
// more appropiate.
//
// Resuming is as easy as pushing new data to the channel returned by Push.
func (l *List) Pause() {
	// this API is preferable over an automated timer in consumeAsyncElements
	// to avoid adding extra logic to an already contentious and hot path
	l.mu.Lock()
	l.sortMatchesList()
	interrupter := l.cfg.interrupter
	l.mu.Unlock()
	interrupter.Interrupt()
}

// PushSync pushes one element to this list and searches for a match on it.
// If data matches the search input, this element is appended to the list and
// this method returns true.
func (l *List) PushSync(b []byte) bool {
	return l.pushData(b, nil, true)
}

// FocusUp moves the focus of the match list up.
func (l *List) FocusUp() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.list.FocusUp()
}

// FocusDown moves the focus of the match list down.
func (l *List) FocusDown() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.list.FocusDown()
}

// FocusStart moves the focus of the match list to the start.
func (l *List) FocusStart() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.list.FocusStart()
}

// FocusEnd moves the focus of the match list to the end.
func (l *List) FocusEnd() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.list.FocusEnd()
}

// Focus returns the match in the list currently in focus.
func (l *List) Focus() (Match, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	node, ok := l.list.Focus()
	if !ok {
		return Match{}, false
	}
	comp := node.Value().(component.WithAttributes)
	return comp.(searchResultComponent).Match, true
}

// SetFocus sets the focus of this List to node.
func (l *List) SetFocus(node component.ListNode) {
	l.mu.Lock()
	defer l.mu.Unlock()
	// Note: due to performance reasons, Match has to be
	// stack allocated and therefore it canot contain
	// a pointer to its component.ListNode, thus forcing
	// this List's API to take both ListNode and Match
	// rendering it somewhat inconsistent.
	// TODO: benchcmp with heap allocated Match exclusively
	// vs stack allocated but also perform holistic benchmark
	// with GC on a real live session.
	l.list.SetFocus(node)
}

// IterateVisible iterates only the visible elements in l.
func (l *List) IterateVisible(fn func(Match)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.list.IterateVisible(func(c component.WithAttributes) {
		fn(c.(searchResultComponent).Match)
	})
}

func (l *List) asyncSearch() {
	if l.cancelSearch != nil {
		l.cancelSearch()
	}

	ctx := context.Background()
	l.searchCtx, l.cancelSearch = context.WithCancel(ctx)

	input := make([][]byte, len(l.input))
	copy(input, l.input)
	searchInput := l.getSearchQuery()
	l.list.Reset()
	go l.handleSearch(l.searchCtx, l.cancelSearch, input, searchInput)
}

// DataReset resets the current data list.
func (l *List) DataReset() {
	l.mu.Lock()
	defer l.mu.Unlock()
	cancel := l.cancelSearch
	if cancel != nil {
		cancel()
	}
	l.input = l.input[:0]
	l.list.Reset()
	l.setFilesCount()
}

// Wait waits for the current search to finish if any and returns.
// It does not wait for any pending data being consumed asyncronously
// via Push.
func (l *List) Wait() {
	l.mu.Lock()
	ctx := l.searchCtx
	l.mu.Unlock()

	if ctx == nil {
		return
	}

	<-ctx.Done()
}

// Cancel cancels the current search if there's any.
func (l *List) Cancel() {
	l.mu.Lock()
	cancel := l.cancelSearch
	l.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
}

func (l *List) drawList(w term.Writer) {
	l.list.Virtual.Draw(w)
}

func (l *List) drawSearchBar(w term.Writer) {
	dirty := l.searchBar.dirty
	if dirty {
		l.resize(l.width, l.height)
	}
	l.searchBar.Virtual.Draw(w)
}

func (l *List) drawMatchCounts(w term.Writer) {
	if l.matchCountBar.width != getMatchCountBarWidth(&l.matchCountBar) {
		l.resize(l.width, l.height)
	}
	l.matchCountBar.Virtual.Draw(w)
}

// Draw satisfies tui.Component
func (l *List) Draw(w term.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.drawList(w)
	l.drawSearchBar(w)
	l.drawMatchCounts(w)
}

// Resize satisfies tui.Component
func (l *List) Resize(width, height int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.resize(width, height)
}

func (l *List) resize(width, height int) {
	l.height = height
	l.width = width
	l.searchBar.dirty = false

	inputHeight := l.inputHeight()

	if height-inputHeight < 2 {
		l.list.Move(term.Coordinates{Y: 0})
		l.list.Virtual.Resize(width, height)
		l.searchBar.Virtual.Resize(0, 0)
		l.matchCountBar.Virtual.Resize(0, 0)
		return
	}

	lenFilesCounter := getMatchCountBarWidth(&l.matchCountBar)
	listHeight := height - inputHeight
	if !l.cfg.bottomSearchBar {
		l.list.Move(term.Coordinates{Y: inputHeight})
		l.list.Virtual.Resize(width, listHeight)

		l.searchBar.Virtual.Move(term.Coordinates{})
		l.searchBar.Virtual.Resize(width, inputHeight)
		resizeMatchCountBar(&l.matchCountBar, inputHeight-1, lenFilesCounter, width)
	} else {
		l.list.Move(term.Coordinates{Y: 0})
		l.list.Virtual.Resize(width, listHeight)

		l.searchBar.Virtual.Move(term.Coordinates{Y: listHeight})
		l.searchBar.Virtual.Resize(width, inputHeight)
		resizeMatchCountBar(&l.matchCountBar, listHeight+inputHeight-1, lenFilesCounter, width)
	}
}

func getMatchCountBarWidth(matchCountBar *matchCounter) int {
	return matchCountBar.Buffer.Columns(0)
}

func (l *List) searchBarWidth() int {
	return l.searchBar.internalRead.Columns(0)
}

func resizeMatchCountBar(matchCountBar *matchCounter, y, lenFilesCounter, width int) {
	if width-lenFilesCounter <= 0 {
		matchCountBar.Virtual.Resize(0, 0)
		return
	}

	matchCountBar.width = lenFilesCounter
	matchCountBar.Move(term.Coordinates{
		Y: y,
		X: width - lenFilesCounter,
	})
	matchCountBar.Virtual.Resize(lenFilesCounter, 1)
}

func (l *List) inputHeight() int {
	// search bar is used as prompt so always return a min of 1,
	// even if buffer is empty
	return int(math.Max(float64(l.searchBar.minInputHeight),
		float64(l.searchBar.Responsive.Height(l.width))))
}

// InputHeight returns the component.Responsive Height of the input search bar.
func (l *List) InputHeight() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.inputHeight()
}

// SetMinInputHeight sets the minimum search input field height.
func (l *List) SetMinInputHeight(height int) {
	if height < 1 {
		panic(fmt.Sprintf("invalid height: %d < 1", height))
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.searchBar.minInputHeight = height
	l.resize(l.width, l.height)
}

// Buffer returns the search input buffer.
func (l *List) Buffer() *cell.Buffer {
	return l.searchBar.syncBuffer
}

// Offset returns this list's current seek offset.
func (l *List) Offset() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.list.Offset()
}

// FocusOffset returns this list's focus index in the underlying list.
func (l *List) FocusOffset() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.list.FocusOffset()
}

// ElementHeight returns the height for each element of this list.
func (l *List) ElementHeight() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.list.ElementHeight()
}

// ElementAt returns the Match at the given position and true, if there's any
// or a zero-valued Match and false if there's none.
func (l *List) ElementAt(pos term.Coordinates) (Match, component.ListNode, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	// NOTE returning a ListNode solely exist to enable usage of SetFocus. If SetFocus
	// ever uses a Match, this method should be removed.
	node, ok := l.list.ElementAt(pos)
	if !ok {
		return Match{}, component.ListNode{}, false
	}
	return node.Value().(searchResultComponent).Match, node, true
}

// Close closes all the resources associated with this List.
func (l *List) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.cancelSearch != nil {
		l.cancelSearch()
	}
	l.list.Reset()
	if l.quitChan == nil {
		return nil
	}
	close(l.quitChan)
	l.quitChan = nil
	return nil
}
