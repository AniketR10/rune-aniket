package cell

import (
	"github.com/ernestrc/go-tui/term"
)

type attrSearcher struct {
	sel  selector
	root Searcher
	attr term.Attributes

	text    string
	matches [][]term.Cell
}

// AttrSearcher returns a Searcher that sets/unsets the search results
// cell attributes upon matching.
func AttrSearcher(
	s Searcher, r Reader, attr term.Attributes,
) SubscriberSearcher {

	as := new(attrSearcher)

	as.root = s
	as.attr = attr
	as.sel = selector{reader: r}

	return as
}

func (s *attrSearcher) OnWillInsert(at term.Coordinates, str string) {
	s.setResultsAttr(term.Attributes{})
}

func (s *attrSearcher) OnWillDelete(from, to term.Coordinates) {
	s.setResultsAttr(term.Attributes{})
}

func (s *attrSearcher) OnDidInsert(from, to term.Coordinates) {
	s.searchMatches(s.text)
}

func (s *attrSearcher) OnDidDelete(start, end term.Coordinates, str string) {
	s.searchMatches(s.text)
}

func (s *attrSearcher) setResultsAttr(attr term.Attributes) {
	for i, match := range s.matches {
		for x := range match {
			s.matches[i][x].Bg = attr.Bg
			s.matches[i][x].Fg = attr.Fg
		}
	}
}

// TODO add benchmark and optimize by just re-searching on line
func (s *attrSearcher) searchMatches(text string) int {
	s.matches = nil
	s.text = text

	n := s.root.Search(text)
	if n == 0 {
		return 0
	}
	slen := len(text)
	for i := 0; i < n; i++ {
		pos, _ := s.root.NextResult()
		toX := pos.X + slen
		cells := s.sel.selectCells(pos, term.Coordinates{Y: pos.Y, X: toX})
		s.matches = append(s.matches, cells[0])
	}

	s.setResultsAttr(s.attr)

	return n
}

func (s *attrSearcher) Search(text string) int {
	s.setResultsAttr(term.Attributes{})
	return s.searchMatches(text)
}

func (s *attrSearcher) PrevResult() (pos term.Coordinates, ok bool) {
	return s.root.PrevResult()
}

func (s *attrSearcher) NextResult() (pos term.Coordinates, ok bool) {
	return s.root.NextResult()
}

func (s *attrSearcher) Result() (pos term.Coordinates, ok bool) {
	return s.root.Result()
}

func (s *attrSearcher) Reset() {
	s.matches = nil
	s.text = ""
	s.root.Reset()
}
