package cell

import (
	"testing"

	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
)

func TestConvertCoordinates(t *testing.T) {
	tsuite := []struct {
		cells [][]term.Cell
		y, x  int
		out   term.Coordinates
	}{
		{nil, 0, 0, term.Coordinates{}},
		{[][]term.Cell{{{Ch: 'a'}}}, 0, 0, term.Coordinates{}},
		{[][]term.Cell{{{Ch: 0}, {Ch: 0}, {Ch: 0}, {Ch: '\t'}, {Ch: 'a'}}}, 0, 1, term.Coordinates{X: 4}},
		{[][]term.Cell{{}, {{Ch: 0}, {Ch: 0}, {Ch: 0}, {Ch: '\t'}, {Ch: 'a'}, {Ch: 0}}}, 1, 1, term.Coordinates{Y: 1, X: 4}},
	}

	for _, tcase := range tsuite {
		out := ConvertCoordinates(tcase.cells, tcase.y, tcase.x)
		assert.Equal(t, tcase.out, out)
	}
}
