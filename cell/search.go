package cell

import (
	"container/list"

	"github.com/ernestrc/fractal/term"
)

// Searcher is an interface that wraps methods to search text in a Reader.
type Searcher interface {
	// Search performs a text search on a Reader. It populates a search list
	// so subsequent calls to PrevResult and NextResult can scroll through the results.
	// The return value represents number of text occurrences found.
	Search(text string) int

	// PrevResult returns the coordinates of the previous result in the Search list
	// and moves the search result pointer. Returns false if there aren't any search
	// matches. Implementors should wrap around and continue to the
	// last match once result list is exhausted.
	PrevResult() (pos term.Coordinates, ok bool)

	// NextResult returns the coordinates of the next result in the Search list
	// and moves the search result pointer. Returns false if there aren't any search
	// matches. Implementors should wrap around and continue to the
	// first match once result list is exhausted.
	NextResult() (pos term.Coordinates, ok bool)

	// Result returns the search pointer's coordinates or false if there is no
	// search result.
	Result() (pos term.Coordinates, ok bool)

	// Reset resets the result list of this Searcher.
	Reset()
}

type simpleSearcher struct {
	reader  Reader
	reslist list.List
	result  *list.Element
}

// NewSimpleSearcher returns a Searcher which uses a simple algorithm to find ALL
// occurrences of the given string upon calling Search.
func NewSimpleSearcher(reader Reader) Searcher {
	s := new(simpleSearcher)
	s.reader = reader
	s.Reset()
	return s
}

func (s *simpleSearcher) Search(text string) int {
	s.reslist.Init()
	s.result = nil

	searchRunes := []rune(text)
	slen := len(searchRunes)
	if slen == 0 {
		return 0
	}

	var pos term.Coordinates
	var o int
	for y, r := range s.reader.RawCells() {
		for x, c := range r {
			if c.Ch != searchRunes[o] {
				o = 0
			} else if o == 0 {
				pos = term.Coordinates{X: x, Y: y}
				o++
			} else {
				o++
			}
			if o == slen {
				s.reslist.PushBack(pos)
				o = 0
			}
		}
		// search is not performed across rows
		o = 0
	}

	return s.reslist.Len()
}

// PrevResult returns the coordinates of the previous result in the Search list.
func (s *simpleSearcher) PrevResult() (pos term.Coordinates, ok bool) {
	if s.result == nil {
		s.result = s.reslist.Back()
	} else if s.result = s.result.Prev(); s.result == nil {
		s.result = s.reslist.Back()
	}

	if s.result == nil {
		return
	}

	pos = s.result.Value.(term.Coordinates)
	ok = true
	return
}

// NextResult returns the coordinates of the next result in the Search list.
func (s *simpleSearcher) NextResult() (pos term.Coordinates, ok bool) {
	if s.result == nil {
		s.result = s.reslist.Front()
	} else if s.result = s.result.Next(); s.result == nil {
		s.result = s.reslist.Front()
	}

	if s.result == nil {
		return
	}

	pos = s.result.Value.(term.Coordinates)
	ok = true
	return
}

// Result returns the current search result's coordinates.
func (s *simpleSearcher) Result() (pos term.Coordinates, ok bool) {
	if s.result == nil {
		ok = false
		return
	}
	pos = s.result.Value.(term.Coordinates)
	ok = true
	return
}

func (s *simpleSearcher) Reset() {
	s.result = nil
	s.reslist.Init()
}
