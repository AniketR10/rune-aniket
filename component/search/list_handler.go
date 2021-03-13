package search

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

type simpleHandler struct {
	*List
	fn func(string)
}

// Handler wraps a List to satisfy tui.Handler.
// It handles enter key by calling fn with the element in focus,
// if there's an element in focus at all.
// It handles esc key by exiting and handles arrow keys up/down
// by scrolling up and down the list.
func Handler(l *List, fn func(string)) tui.Handler {
	return simpleHandler{List: l, fn: fn}
}

func (s simpleHandler) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type != term.EventKey {
		return
	}

	switch ev.Key {
	case term.KeyEnter:
		item, ok := s.Focus()
		if ok {
			handled = true
			exit = true
			s.fn(string(item))
		}
	case term.KeyEsc:
		exit = true
	case term.KeyArrowDown:
		handled = s.FocusDown()
	case term.KeyArrowUp:
		handled = s.FocusUp()
	case term.KeySpace:
		ev.Ch = ' '
	case term.KeyBackspace:
		fallthrough
	case term.KeyBackspace2:
		handled = s.SearchQueryDelete()
	}

	if ev.Ch != 0 {
		s.SearchQueryWrite(ev.Ch)
		handled = true
	}

	return
}

func (s simpleHandler) Cursor() (pos term.Coordinates, show bool) {
	return term.Coordinates{X: s.SearchQueryLen()}, true
}

func (s simpleHandler) Man() tui.Manual {
	return tui.Manual{}
}
