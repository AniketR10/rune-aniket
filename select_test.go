package fractal

import (
	"reflect"
	"testing"

	"github.com/nsf/termbox-go"
)

func TestSelect(t *testing.T) {
	var buf Buffer
	str := "hello\n\tworld\n\nitsme"
	buf.tabspaces = 4
	buf.WriteString(str)

	testCases := []selectCase{
		{
			from: Coordinates{},
			to:   Coordinates{X: 1, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}},
			},
		},
		{
			from: Coordinates{X: 2, Y: 0},
			to:   Coordinates{X: 1, Y: 1},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]termbox.Cell{{}, {}, {}},
			},
		},
		{
			from: Coordinates{X: 2, Y: 0},
			to:   Coordinates{X: 1, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'e'}, {Ch: 'l'}},
			},
		},
		{
			from: Coordinates{X: 0, Y: 1},
			to:   Coordinates{X: 8, Y: 1},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
			},
		},
		{
			from: Coordinates{X: 2, Y: 0},
			to:   Coordinates{X: 4, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
			},
		},
		{
			from: Coordinates{X: 2, Y: 0},
			to:   Coordinates{X: 7, Y: 1},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}},
			},
		},
		{
			from: Coordinates{X: 7, Y: 1},
			to:   Coordinates{X: 2, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}},
			},
		},
		{
			from: Coordinates{X: 4, Y: 0},
			to:   Coordinates{X: 4, Y: 3},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'o'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				[]termbox.Cell{},
				[]termbox.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: Coordinates{X: 0, Y: 2},
			to:   Coordinates{X: 4, Y: 3},
			expected: [][]termbox.Cell{
				[]termbox.Cell{},
				[]termbox.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: Coordinates{X: 0, Y: 2},
			to:   Coordinates{X: 5, Y: 3},
			expected: [][]termbox.Cell{
				[]termbox.Cell{},
				[]termbox.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: Coordinates{X: 0, Y: 2},
			to:   Coordinates{X: 4, Y: 4},
			expected: [][]termbox.Cell{
				[]termbox.Cell{},
				[]termbox.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
	}

	for _, tcase := range testCases {
		selection := Select(buf.RawCells(), tcase.from, tcase.to)
		if !reflect.DeepEqual(selection, tcase.expected) {
			t.Errorf("expected %q found %q", toString(tcase.expected), toString(selection))
		}
	}
}

func TestSelectLine(t *testing.T) {
	var buf Buffer
	str := "hello\n\tworld\n\nitsme"
	buf.WriteString(str)
	buf.tabspaces = 4

	testCases := []selectCase{
		{
			from: Coordinates{},
			to:   Coordinates{X: 1, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
			},
		},
		{
			from: Coordinates{X: 2, Y: 0},
			to:   Coordinates{X: 4, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
			},
		},
		{
			from: Coordinates{X: 2, Y: 0},
			to:   Coordinates{X: 7, Y: 1},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
			},
		},
		{
			from: Coordinates{X: 0, Y: 2},
			to:   Coordinates{X: 2, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				[]termbox.Cell{},
			},
		},
		{
			to:   Coordinates{X: 2, Y: 0},
			from: Coordinates{X: 10, Y: 2},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				[]termbox.Cell{},
			},
		},
		{
			to:   Coordinates{X: 2, Y: 0},
			from: Coordinates{X: 0, Y: 10},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				[]termbox.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
	}

	for _, tcase := range testCases {
		selection := SelectLine(buf.RawCells(), tcase.from, tcase.to)
		if toString(selection) != toString(tcase.expected) {
			t.Errorf("expected %q found %q", toString(tcase.expected), toString(selection))
		}
	}
}

func TestSelectBlock(t *testing.T) {
	var buf Buffer
	str := "hello\n\tworld\n\nitsme\n\n\nhi"
	buf.WriteString(str)
	buf.tabspaces = 4

	testCases := []selectCase{
		{
			from: Coordinates{},
			to:   Coordinates{X: 1, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}},
			},
		},
		{
			from: Coordinates{X: 0, Y: 0},
			to:   Coordinates{X: 3, Y: 1},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}},
			},
		},
		{
			from: Coordinates{X: 3, Y: 1},
			to:   Coordinates{X: 0, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}},
			},
		},
		{
			from: Coordinates{X: 3, Y: 3},
			to:   Coordinates{X: 0, Y: 0},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}},
				[]termbox.Cell{{}, {}, {}, {Ch: '\t'}},
				[]termbox.Cell{},
				[]termbox.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}},
			},
		},
		{
			from: Coordinates{X: 4, Y: 0},
			to:   Coordinates{X: 2, Y: 6},
			expected: [][]termbox.Cell{
				[]termbox.Cell{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]termbox.Cell{{}, {Ch: '\t'}, {Ch: 'w'}},
				[]termbox.Cell{},
				[]termbox.Cell{{Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
				[]termbox.Cell{},
				[]termbox.Cell{},
				[]termbox.Cell{},
			},
		},
	}

	for _, tcase := range testCases {
		selection := SelectBlock(buf.RawCells(), tcase.from, tcase.to)
		if !reflect.DeepEqual(selection, tcase.expected) {
			t.Errorf("expected %q found %q", toString(tcase.expected), toString(selection))
		}
	}
}
