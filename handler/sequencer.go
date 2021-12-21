package handler

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ernestrc/go-tui/term"
)

// Sequence represents a term.EventKey sequence. For matching purposes,
// only Event.Mod, Event.Key and Event.Ch are considered,
// the rest of fields are ignored.
type Sequence struct {
	First term.Event
	Last  term.Event
}

// Sequencer is a key event sequencer which detects sequences of (2) term.EventKey
// events that are triggered within a specified time.
type Sequencer struct {
	interests map[term.Event]map[term.Event]struct{}
	timeout   time.Duration

	first    term.Event
	firstCtx context.Context
	ctxClean func()
}

// NewSequencer allocates storage for a new Sequencer and initalizes it.
func NewSequencer(interests []Sequence, timeout time.Duration) *Sequencer {
	ret := new(Sequencer)
	ret.Init(interests, timeout)
	return ret
}

func cleanEvent(ev term.Event) term.Event {
	return term.Event{
		Mod: ev.Mod,
		Key: ev.Key,
		Ch:  ev.Ch,
	}
}

// Init initializes this sequencer with the given interests and a timeout.
func (s *Sequencer) Init(interests []Sequence, timeout time.Duration) {
	s.timeout = timeout
	s.interests = make(map[term.Event]map[term.Event]struct{})
	for _, i := range interests {
		first, last := cleanEvent(i.First), cleanEvent(i.Last)
		if _, ok := s.interests[first]; !ok {
			s.interests[first] = make(map[term.Event]struct{})
		}
		s.interests[first][last] = struct{}{}
	}
	// initialize now with canceled context so we don't need to check if ctxClean
	// or firstCtx has been initialized
	s.firstCtx, s.ctxClean = context.WithCancel(context.Background())
	s.ctxClean()
}

// Handle processes ev and returns whether there's a match or not,
// and if so, which sequence was processed. It ignores events that are not
// of Type term.EventKey.
func (s *Sequencer) Handle(ev term.Event) (seq Sequence, match bool) {
	if ev.Type != term.EventKey {
		return
	}

	first := s.first
	firstCtx := s.firstCtx
	ctxClean := s.ctxClean
	last := cleanEvent(ev)

	defer ctxClean()
	s.firstCtx, s.ctxClean = context.WithTimeout(context.Background(), s.timeout)
	s.first = last

	select {
	case <-firstCtx.Done():
		return
	default:
	}

	m, ok := s.interests[first]
	if !ok {
		return
	}

	_, ok = m[last]
	if !ok {
		return
	}

	// first is consumed if match is found
	s.Reset()
	seq = Sequence{First: first, Last: last}
	match = true
	return
}

// Reset resets this Sequencer such that the next event passed
// to Handle is considered as a first event.
func (s *Sequencer) Reset() {
	s.ctxClean()
}

// ParseSequence parses str into a Sequence or returns
// error if it fails to parse it. This function is not case sensitive.
// It accepts two key string representation, as they would be individually
// parsed by term.ParseKey: i.e. <c-x><c-p>, f<c-p>, <c-x>t, gf
func ParseSequence(str string) (Sequence, error) {
	runestr := []rune(str)
	switch len(runestr) {
	case 0, 1, 3, 4, 5:
		return Sequence{}, errors.New("invalid sequence")
	case 2:
		return Sequence{
			First: term.Event{Type: term.EventKey, Ch: runestr[0]},
			Last:  term.Event{Type: term.EventKey, Ch: runestr[1]},
		}, nil
	default:
		idxGt := strings.IndexRune(str, '>')
		idxLt := strings.IndexRune(str, '<')

		if idxLt > idxGt {
			return Sequence{}, errors.New("invalid sequence")
		}

		ev, err := term.ParseKey(string(runestr[idxLt : idxGt+1]))
		if err != nil {
			return Sequence{}, err
		}

		ret := Sequence{}
		switch idxLt {
		case -1:
			return Sequence{}, errors.New("invalid sequence")
		case 0:
			ret.First = ev
			ret.Last, err = term.ParseKey(string(runestr[idxGt+1:]))
		default:
			ret.Last = ev
			ret.First, err = term.ParseKey(string(runestr[:idxLt]))
		}
		if err != nil {
			return Sequence{}, err
		}
		return ret, nil
	}
}
