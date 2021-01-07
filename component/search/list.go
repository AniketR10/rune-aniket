package search

import (
	"context"
	"fmt"
	"sync"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/term"
	fzf "github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
)

const (
	numElementsToReDrawAt    = 128
	maxNumElementsToReDrawAt = 1024 * 32
)

// internal representation of ListConfig
type listConfig struct {
	matchedTextAttr term.Attributes
	matchCountAttr  term.Attributes
	searchBaseAttr  term.Attributes
	textAttr        term.Attributes
	focusAttr       term.Attributes
	algo            fzf.Algo
	interrupt       func()
	caseSensitive   bool
	searchBase      string
}

// List is a collection of elements that can be interactively searched.
type List struct {
	mu           sync.Mutex
	quitChan     chan struct{}
	dataChan     chan []byte
	input        [][]byte
	searchCtx    context.Context
	cancelSearch func()
	height       int

	cfg listConfig

	searchBar struct {
		cell.Buffer
		component.Scroll
		component.Virtual
	}

	matchCountBar struct {
		cell.Buffer
		component.Scroll
		component.Virtual
	}
	list struct {
		component.Virtual
		component.FocusList
	}
}

type searchResultComponent struct {
	*component.AttrSetter
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
	l.searchBar.Buffer.Init()
	l.searchBar.Scroll.InitWithBuffer(&l.searchBar.Buffer)
	l.searchBar.C = &l.searchBar.Scroll
	l.matchCountBar.Buffer.Init()
	l.matchCountBar.Scroll.InitWithBuffer(&l.matchCountBar.Buffer)
	l.matchCountBar.C = &l.matchCountBar.Scroll
	l.list.C = &l.list.FocusList

	if l.cfg.searchBase != "" {
		l.searchBar.InsertStringWithAttr(term.Coordinates{},
			l.cfg.searchBase, l.cfg.searchBaseAttr)
	}

	l.list.InitWithAttr(l.cfg.textAttr, l.cfg.focusAttr)
	l.quitChan = make(chan struct{})
	l.dataChan = make(chan []byte)
	l.setFilesCount()

	go l.consumeAsyncElements()
}

func (l *List) addMatch(match Match, tokens *[]int) {
	matchText := match.data
	b := component.NewSpan(component.String(string(matchText)), component.SpanConfig{
		ContentAlignment: component.SpanAlignmentLeft,
	})

	comp := component.WithAttrSetter(b)
	if tokens != nil {
		for _, t := range *tokens {
			comp.SetAttrAt(term.Coordinates{X: t}, l.cfg.matchedTextAttr)
		}
	}

	resComp := searchResultComponent{
		AttrSetter: comp,
		Match:      match,
	}

	l.list.PushBack(resComp)
}

func sortByResultScore(a, b component.WithAttributes) bool {
	ab := a.(searchResultComponent)
	bb := b.(searchResultComponent)
	if ab.Match.res.Score == bb.Match.res.Score {
		return len(ab.Match.data) < len(bb.Match.data)
	}
	return ab.Match.res.Score > bb.Match.res.Score
}

func (l *List) getSearchQuery() string {
	str := l.searchBar.Buffer.String()
	return str[len(l.cfg.searchBase):]
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

func (l *List) setFilesCount() {
	l.matchCountBar.Reset()
	str := fmt.Sprintf("%d/%d", l.list.Len(), len(l.input))
	l.matchCountBar.InsertStringWithAttr(term.Coordinates{}, str, l.cfg.matchCountAttr)
}

func (l *List) sortMatchesList() {
	l.list.Sort(sortByResultScore)
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
		l.addMatch(Match{data: data}, nil)
		matched = true
		if sortList {
			l.setFilesCount()
		}
		return
	}

	search(l.cfg.algo, linebuf[:], searchInput, slab, l.cfg.caseSensitive,
		func(match Match, tokens *[]int) bool {
			l.addMatch(match, tokens)
			matched = true
			return false
		})

	if sortList {
		l.sortMatchesList()
	}

	return
}

func (l *List) consumeAsyncElements() {
	// to handle very large searches, we redraw only every
	// numElementsToReDrawAt to start with, and we double this
	// number every time until we earch maxNumElementsToReDrawAt,
	// at which point we redraw every maxNumElementsToReDrawAt.
	redrawAt := numElementsToReDrawAt
	slab := makeSlab()
	for i := 0; ; i++ {
		select {
		case data, ok := <-l.dataChan:
			if !ok {
				l.mu.Lock()
				if len(l.getSearchQuery()) == 0 {
					l.setFilesCount()
				} else {
					l.sortMatchesList()
				}
				l.mu.Unlock()
				l.cfg.interrupt()
				return
			}
			l.mu.Lock()
			height := l.height
			l.mu.Unlock()
			redraw := i == height || i%redrawAt == 0
			l.pushData(data, slab, redraw)
			if redraw {
				l.cfg.interrupt()
				if redrawAt < maxNumElementsToReDrawAt {
					redrawAt *= 2
				}
			}
		case <-l.quitChan:
			l.mu.Lock()
			defer l.mu.Unlock()
			l.input = nil
			return
		}
	}
}

func (l *List) handleSearch(ctx context.Context) {
	l.mu.Lock()
	l.list.Reset()
	input := l.input
	searchInput := l.getSearchQuery()
	l.mu.Unlock()

	slab := makeSlab()
	search(l.cfg.algo, input, searchInput, slab, l.cfg.caseSensitive,
		func(match Match, tokens *[]int) bool {
			select {
			case <-ctx.Done():
				return false
			default:
				l.mu.Lock()
				defer l.mu.Unlock()
				l.addMatch(match, tokens)
				return true
			}
		})

	select {
	case <-ctx.Done():
		return
	default:
	}

	l.mu.Lock()
	l.sortMatchesList()
	l.cancelSearch()
	l.mu.Unlock()
	l.cfg.interrupt()
}

// Push returns a channel that can be used to push data to this list asynchronously.
// Clients can and should call close on the channel, once no more data is expected.
// See PushSync for more details.
func (l *List) Push() chan<- []byte {
	return l.dataChan
}

// PushSync pushes one element to this list and searches for a match on it.
// If data matches the search input, this element is appended to the list and
// this method returns true.
func (l *List) PushSync(b []byte) (match bool) {
	return l.pushData(b, nil, true)
}

// FocusUp moves the focus of the match list up.
func (l *List) FocusUp() bool {
	return l.list.FocusUp()
}

// FocusDown moves the focus of the match list down.
func (l *List) FocusDown() bool {
	return l.list.FocusDown()
}

// FocusStart moves the focus of the match list to the start.
func (l *List) FocusStart() bool {
	return l.list.FocusStart()
}

// FocusEnd moves the focus of the match list to the end.
func (l *List) FocusEnd() bool {
	return l.list.FocusEnd()
}

// Focus returns the match in the list currently in focus.
func (l *List) Focus() ([]byte, bool) {
	node, ok := l.list.Focus()
	if !ok {
		return nil, false
	}
	return node.Value().(searchResultComponent).data, true
}

func (l *List) asyncSearch() {
	if l.cancelSearch != nil {
		l.cancelSearch()
	}

	ctx := context.Background()
	l.searchCtx, l.cancelSearch = context.WithCancel(ctx)

	go l.handleSearch(l.searchCtx)
}

// SearchQueryLen returns the length of the current search query.
func (l *List) SearchQueryLen() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	return l.searchBar.Columns(0)
}

// SearchQueryWrite adds r to the search query, canceling the
// previous search if any. It asynchronously starts a new search.
func (l *List) SearchQueryWrite(r rune) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.searchBar.WriteString(string(r))
	l.asyncSearch()
}

// SearchQueryDelete removes the last rune added to the search query,
// canceling the previous search if any. It asynchronously performs
// starts a new search.
func (l *List) SearchQueryDelete() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	size := l.searchBar.Columns(0)
	if size > len(l.cfg.searchBase) {
		l.searchBar.Buffer.DeleteCell(term.Coordinates{X: size - 1, Y: 0})
		l.asyncSearch()
		return true
	}
	return false
}

// Wait waits for the current search to finish if any and returns.
func (l *List) Wait() {
	l.mu.Lock()
	ctx := l.searchCtx
	l.mu.Unlock()

	<-ctx.Done()
}

// Draw satisfies tui.Component
func (l *List) Draw(w term.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.list.Virtual.Draw(w)
	l.searchBar.Virtual.Draw(w)
	l.matchCountBar.Virtual.Draw(w)
}

// Resize satisfies tui.Component
func (l *List) Resize(width, height int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.height = height

	if height <= 1 {
		l.list.Move(term.Coordinates{Y: 0})
		l.list.Virtual.Resize(width, height)
		l.searchBar.Virtual.Resize(0, 0)
		l.matchCountBar.Virtual.Resize(0, 0)
		return
	}

	l.list.Move(term.Coordinates{Y: 1})
	l.list.Virtual.Resize(width, height-1)

	l.searchBar.Move(term.Coordinates{})
	l.searchBar.Virtual.Resize(width, 1)

	lenFilesCounter := l.matchCountBar.Buffer.Columns(0)
	if width-lenFilesCounter <= 0 {
		l.matchCountBar.Virtual.Resize(0, 0)
		return
	}

	l.matchCountBar.Move(term.Coordinates{X: width - lenFilesCounter})
	l.matchCountBar.Virtual.Resize(lenFilesCounter, 1)
}

// Close closes all the resources associated with this List.
func (l *List) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.cancelSearch != nil {
		l.cancelSearch()
	}
	close(l.quitChan)
	l.list.Reset()
	return nil
}
