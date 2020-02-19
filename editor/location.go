package editor

import "github.com/ernestrc/go-tui/term"

// LocationList is the interface that groups Prev and Next
// location methods to fetch the previous and next item respectively.
//
// Both Prev/Next return false if reached either end or beginning of list.
//
// Current returns the current result. If there are no results in the list
// then Current returns false.
type LocationList interface {
	Current() (Location, bool)
	Prev() (Location, bool)
	Next() (Location, bool)
}

// Location represents a content location in a Editor's buffer.
type Location struct {
	From, To term.Coordinates
	Attr     term.Attributes
}
