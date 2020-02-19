package editor

type testLocationList struct {
	curr      int
	locations []Location
}

func (t *testLocationList) Current() (Location, bool) {
	if len(t.locations) == 0 {
		return Location{}, false
	}
	return t.locations[t.curr], true
}

func (t *testLocationList) Prev() (Location, bool) {
	if t.curr == 0 {
		return Location{}, false
	}
	t.curr--
	return t.locations[t.curr], true
}

func (t *testLocationList) Next() (Location, bool) {
	if t.curr == len(t.locations)-1 {
		return Location{}, false
	}
	t.curr++
	return t.locations[t.curr], true
}
