package text

import (
	"testing"

	"github.com/ernestrc/tcell/v3"
	"github.com/stretchr/testify/assert"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/term"
)

var (
	loc1 = textapi.Location{
		To:   term.Coordinates{X: 1, Y: 3},
		Attr: term.Attributes{Attrs: tcell.AttrBold},
	}
	loc2 = textapi.Location{
		From:    term.Coordinates{X: 1, Y: 3},
		Attr:    term.Attributes{Fg: tcell.ColorBlack, Bg: tcell.ColorGreen},
		Message: "wsb: hold BBBY",
	}
	loc3 = textapi.Location{}
)

func resetLocationList(l LocationList) {
	for {
		_, ok := l.Prev()
		if !ok {
			break
		}
	}
}

func assertLocation(t *testing.T, l LocationList, idx int, loca textapi.Location) {
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
	l := LocationSlice([]textapi.Location{
		loc2,
	})

	assertLocation(t, l, 0, loc2)
	assertLocationListLen(t, l, 1)

	l = LocationSlice([]textapi.Location{
		loc1,
		loc2,
		loc3,
	})

	assertLocation(t, l, 0, loc1)
	assertLocation(t, l, 1, loc2)
	assertLocation(t, l, 2, loc3)
	assertLocationListLen(t, l, 3)

	l = LocationSlice([]textapi.Location{})
	assertLocationListLen(t, l, 0)

	l = LocationSlice(nil)
	assertLocationListLen(t, l, 0)
}
