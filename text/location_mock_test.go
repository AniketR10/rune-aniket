package text

import textapi "unstable.build/go-tui/api/text"

type testLocationList struct {
	curr      int
	locations []textapi.Location
}

func (t *testLocationList) Current() (textapi.Location, bool) {
	if len(t.locations) == 0 {
		return textapi.Location{}, false
	}
	return t.locations[t.curr], true
}

func (t *testLocationList) Prev() (textapi.Location, bool) {
	if t.curr == 0 {
		return textapi.Location{}, false
	}
	t.curr--
	return t.locations[t.curr], true
}

func (t *testLocationList) Next() (textapi.Location, bool) {
	if t.curr == len(t.locations)-1 {
		return textapi.Location{}, false
	}
	t.curr++
	return t.locations[t.curr], true
}
