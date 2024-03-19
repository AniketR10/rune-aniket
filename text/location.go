package text

import (
	"math"

	textapi "unstable.build/go-tui/api/text"
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
	if scroll.Width() == 0 {
		return // avoid division by 0 in ScrollToWindowCoordinates
	}

	offset := scroll.Offset()
	height := scroll.SizeHeight()
	buffer := scroll.Buffer()

	var wraps int
	for _, count := range scroll.Wraps() {
		wraps += count
	}
	// this might add some extra calls to UnionAttributes that are noop but
	// it's way more efficient that calculating screen coordinates on every
	// location.
	minY := offset.Y - wraps
	maxY := offset.Y + height
	for _, loc := range locations {
		fromAtScroll := loc.From
		toAtScroll := loc.To
		if fromAtScroll.Y >= maxY || fromAtScroll.Y < minY {
			continue
		}

		fromAtScrollX := fromAtScroll.X
		toAtScroll.Y = int(math.Min(float64(buffer.Rows()-1), float64(toAtScroll.Y)))
		for y := fromAtScroll.Y; y < toAtScroll.Y; y++ {
			toX := buffer.Columns(y)
			for x := fromAtScrollX; x < toX; x++ {
				posAtScreen := scroll.ScrollToWindowCoordinates(term.Coordinates{Y: y, X: x})
				if posAtScreen.X >= scroll.Width() {
					break
				}
				if posAtScreen.X < 0 {
					continue
				}
				w.UnionAttributes(posAtScreen, loc.Attr)
			}
			fromAtScrollX = 0
		}

		toX := toAtScroll.X
		for x := fromAtScrollX; x < toX; x++ {
			posAtScreen := scroll.ScrollToWindowCoordinates(term.Coordinates{Y: toAtScroll.Y, X: x})
			if posAtScreen.X >= scroll.Width() {
				break
			}
			if posAtScreen.X < 0 {
				continue
			}
			w.UnionAttributes(posAtScreen, loc.Attr)
		}
	}
}
