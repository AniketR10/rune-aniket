package cell

import (
	"testing"

	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertCoordinates(t *testing.T) {
	tsuite := []struct {
		cells [][]term.Cell
		y, x  int
		out   term.Coordinates
		ok    bool
	}{
		{nil, 0, 0, term.Coordinates{}, false},
		{[][]term.Cell{{{Ch: 'a'}}}, 0, 0, term.Coordinates{}, true},
		{[][]term.Cell{{{Ch: 0}, {Ch: 0}, {Ch: 0}, {Ch: '\t'}, {Ch: 'a'}}}, 0, 1, term.Coordinates{X: 4}, true},
		{[][]term.Cell{{}, {{Ch: 0}, {Ch: 0}, {Ch: 0}, {Ch: '\t'}, {Ch: 'a'}, {Ch: 0}}}, 1, 1, term.Coordinates{Y: 1, X: 4}, true},
	}

	for _, tcase := range tsuite {
		out, ok := ConvertRuneCoordinates(tcase.cells, tcase.y, tcase.x)
		require.Equal(t, tcase.ok, ok)
		assert.Equal(t, tcase.out, out)
	}
	for _, tcase := range tsuite {
		y, x, ok := ConvertTermCoordinates(tcase.cells, tcase.out)
		require.Equal(t, tcase.ok, ok)
		assert.Equal(t, tcase.x, x)
		assert.Equal(t, tcase.y, y)
	}
}
