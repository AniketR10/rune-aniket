package api

import "unstable.build/go-tui/term"

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
	Message  string
}

type sliceLocations struct {
	curr int
	in   []Location
}

func (s *sliceLocations) Current() (Location, bool) {
	if s.curr >= len(s.in) || s.curr < 0 {
		return Location{}, false
	}

	return s.in[s.curr], true
}

func (s *sliceLocations) Prev() (Location, bool) {
	if s.curr-1 < 0 {
		return Location{}, false
	}
	s.curr--
	return s.in[s.curr], true
}

func (s *sliceLocations) Next() (Location, bool) {
	if s.curr+1 >= len(s.in) {
		return Location{}, false
	}
	s.curr++
	return s.in[s.curr], true
}

// LocationSlice returns a LocationList based on in
func LocationSlice(in []Location) LocationList {
	return &sliceLocations{in: in}
}
