package cell

import "github.com/ernestrc/fractal/term"

type resetStatefulSearcher struct {
	pub      PublisherReader
	searcher Searcher
	text     []rune
	stale    bool
}

// ResetSubscriberSearcher returns a SubscriberSearcher which
// calls Reset on searcher when there are updates to the buffer
// and re-runs Search upon calls to Next/PrevSearchResult.
func ResetSubscriberSearcher(
	r PublisherReader, searcher Searcher,
) SubscriberSearcher {
	ss := &resetStatefulSearcher{
		pub:      r,
		searcher: searcher,
	}
	r.Subscribe(ss)
	return ss
}

func (s *resetStatefulSearcher) OnWillInsert(
	at term.Coordinates, str string,
) {
}

func (s *resetStatefulSearcher) OnDidInsert(
	from, to term.Coordinates,
) {
	s.searcher.Reset()
	s.stale = true
}

func (s *resetStatefulSearcher) OnWillDelete(
	start, end term.Coordinates,
) {
}

func (s *resetStatefulSearcher) OnDidDelete(
	start, end term.Coordinates, str string,
) {
	s.searcher.Reset()
	s.stale = true
}

func (s *resetStatefulSearcher) Search(text string) int {
	s.text = []rune(text)
	s.stale = false
	return s.searcher.Search(text)
}

func (s *resetStatefulSearcher) fetchResult(
	fetch func(Searcher) (term.Coordinates, bool),
) (pos term.Coordinates, ok bool) {
	if s.text == nil {
		return
	}
	if !s.stale {
		return fetch(s.searcher)
	}
	s.stale = false
	n := s.searcher.Search(string(s.text))
	if n == 0 {
		return
	}
	ok = true

	return fetch(s.searcher)
}

func (s *resetStatefulSearcher) PrevResult() (
	pos term.Coordinates, ok bool,
) {
	return s.fetchResult((Searcher).PrevResult)
}

func (s *resetStatefulSearcher) NextResult() (
	pos term.Coordinates, ok bool,
) {
	return s.fetchResult((Searcher).NextResult)
}

func (s *resetStatefulSearcher) Result() (
	pos term.Coordinates, ok bool,
) {
	return s.fetchResult((Searcher).Result)
}

func (s *resetStatefulSearcher) Reset() {
	s.text = nil
	s.searcher.Reset()
	s.stale = false
}

func (s *resetStatefulSearcher) Unsubscribe() {
	if s.pub == nil {
		return
	}
	s.pub.Unsubscribe(s)
	s.pub = nil
}
