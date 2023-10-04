package extension

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/term"
)

func TestWordURI(t *testing.T) {
	var content = `Love in your heart wasn't put there to stay.
file://tato+a:pass@redbaycoffee.com/tmp/a
`
	tsuite := []struct {
		in      term.Coordinates
		wantOut string
	}{
		{term.Coordinates{}, "Love"},
		{term.Coordinates{X: 4}, ""},
		{term.Coordinates{X: 6}, "in"},
		{term.Coordinates{Y: 1, X: 0}, "file://tato+a:pass@redbaycoffee.com/tmp/a"},
		{term.Coordinates{Y: 1, X: 4}, "file://tato+a:pass@redbaycoffee.com/tmp/a"},
		{term.Coordinates{Y: 1, X: 5}, "file://tato+a:pass@redbaycoffee.com/tmp/a"},
		{term.Coordinates{Y: 1, X: 6}, "file://tato+a:pass@redbaycoffee.com/tmp/a"},
		{term.Coordinates{Y: 1, X: 7}, "file://tato+a:pass@redbaycoffee.com/tmp/a"},
		{term.Coordinates{Y: 1, X: 17}, "file://tato+a:pass@redbaycoffee.com/tmp/a"},
		{term.Coordinates{Y: 1, X: 33}, "file://tato+a:pass@redbaycoffee.com/tmp/a"},
	}

	for i, tcase := range tsuite {
		t.Run(fmt.Sprintf("test case %d: %v", i, tcase.in), func(t *testing.T) {
			f := newFile(content)
			f.MoveToScroll(tcase.in)
			assert.Equal(t, tcase.wantOut, f.uriAtCursor())
		})
	}
}
