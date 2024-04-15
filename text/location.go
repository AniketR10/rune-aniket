package text

import (
	textapi "unstable.build/go-tui/api/text"
)

// LocationList is the interface that groups Prev and Next
// location methods to fetch the previous and next item respectively.
//
// Both Prev/Next return false if reached either end or beginning of list.
//
// Current returns the current result. If there are no results in the list
// then Current returns false.
type LocationList interface {
	Current() (textapi.Location, bool)
	Prev() (textapi.Location, bool)
	Next() (textapi.Location, bool)
}

// LocationSlice returns a LocationList based on in
func LocationSlice(in []textapi.Location) LocationList {
	return &sliceLocations{in: in}
}

type sliceLocations struct {
	curr int
	in   []textapi.Location
}

func (s *sliceLocations) Current() (textapi.Location, bool) {
	if s.curr >= len(s.in) || s.curr < 0 {
		return textapi.Location{}, false
	}

	return s.in[s.curr], true
}

func (s *sliceLocations) Prev() (textapi.Location, bool) {
	if s.curr-1 < 0 {
		return textapi.Location{}, false
	}
	s.curr--
	return s.in[s.curr], true
}

func (s *sliceLocations) Next() (textapi.Location, bool) {
	if s.curr+1 >= len(s.in) {
		return textapi.Location{}, false
	}
	s.curr++
	return s.in[s.curr], true
}
