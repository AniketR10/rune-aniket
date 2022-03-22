package text

import (
	"testing"

	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
)

var (
	loc1 = Location{
		To:   term.Coordinates{X: 1, Y: 3},
		Attr: term.Attributes{Fg: term.AttrBold},
	}
	loc2 = Location{
		From:    term.Coordinates{X: 1, Y: 3},
		Attr:    term.Attributes{Fg: term.ColorBlack, Bg: term.ColorGreen},
		Message: "wsb: hold BBBY",
	}
	loc3 = Location{}
)

func resetLocationList(l LocationList) {
	for {
		_, ok := l.Prev()
		if !ok {
			break
		}
	}
}

func assertLocation(t *testing.T, l LocationList, idx int, loca Location) {
	resetLocationList(l)
	var i int
	for loc, ok := l.Current(); ok; loc, ok = l.Next() {
		if idx == i {
			assert.Equal(t, loca, loc)
			return
		}
		i++
	}
}
func assertLocationListLen(t *testing.T, l LocationList, length int) {
	resetLocationList(l)
	var i int
	for _, ok := l.Current(); ok; _, ok = l.Next() {
		i++
	}
	assert.Equal(t, length, i)
}

func TestLocationSlice(t *testing.T) {
	l := LocationSlice([]Location{
		loc2,
	})

	assertLocation(t, l, 0, loc2)
	assertLocationListLen(t, l, 1)

	l = LocationSlice([]Location{
		loc1,
		loc2,
		loc3,
	})

	assertLocation(t, l, 0, loc1)
	assertLocation(t, l, 1, loc2)
	assertLocation(t, l, 2, loc3)
	assertLocationListLen(t, l, 3)

	l = LocationSlice([]Location{})
	assertLocationListLen(t, l, 0)

	l = LocationSlice(nil)
	assertLocationListLen(t, l, 0)
}
