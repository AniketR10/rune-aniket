package text

import (
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
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

// LocationSlice returns a LocationList based on in
func LocationSlice(in []textapi.Location) LocationList {
	return &sliceLocations{in: in}
}

// DrawLocations draws the given locations using the given writer.
// This is intended to be used alongside Scroll's Draw method.
func DrawLocations(locations []textapi.Location, scroll *component.Scroll, w term.Writer) {
	for _, loc := range locations {
		fromAtScroll := loc.From
		toAtScroll := loc.To
		fromAtScroll, toAtScroll = cell.SortFromTo(fromAtScroll, toAtScroll)
		fromAtScreen := scroll.ScrollToWindowCoordinates(fromAtScroll)
		if fromAtScreen.Y >= scroll.SizeHeight() || fromAtScreen.Y < 0 {
			continue
		}

		fromAtScrollX := fromAtScroll.X
		for y := fromAtScroll.Y; y <= toAtScroll.Y; y++ {
			var toX int
			if y == toAtScroll.Y {
				toX = toAtScroll.X
			} else {
				toX = scroll.Buffer().Columns(y)
			}
			for x := fromAtScrollX; x < toX; x++ {
				posAtScroll := term.Coordinates{Y: y, X: x}
				posAtScreen := scroll.ScrollToWindowCoordinates(posAtScroll)
				if posAtScreen.X >= scroll.Width() || posAtScreen.X < 0 {
					continue
				}
				w.UnionAttributes(posAtScreen, loc.Attr)
			}
			fromAtScrollX = 0
		}
	}
}
