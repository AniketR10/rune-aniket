package cell

import (
	"reflect"
	"testing"

	"github.com/ernestrc/fractal/term"
)

func TestSelect(t *testing.T) {
	var buf Buffer
	str := `hello
	world

itsme`
	buf.WriteString(str)

	testCases := []selectCase{
		{
			from: term.Coordinates{},
			to:   term.Coordinates{X: 1, Y: 0},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'h'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{X: 2, Y: 0},
			to:   term.Coordinates{X: 1, Y: 1},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]term.Cell{{}, {}},
			},
		},
		{
			from: term.Coordinates{X: 2, Y: 0},
			to:   term.Coordinates{X: 1, Y: 0},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'e'}, {Ch: 'l'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 1},
			to:   term.Coordinates{X: 8, Y: 1},
			expected: [][]term.Cell{
				[]term.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
			},
		},
		{
			from: term.Coordinates{X: 2, Y: 0},
			to:   term.Coordinates{X: 4, Y: 0},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
			},
		},
		{
			from: term.Coordinates{X: 2, Y: 0},
			to:   term.Coordinates{X: 7, Y: 1},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]term.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}},
			},
		},
		{
			from: term.Coordinates{X: 7, Y: 1},
			to:   term.Coordinates{X: 2, Y: 0},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]term.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}},
			},
		},
		{
			from: term.Coordinates{X: 4, Y: 0},
			to:   term.Coordinates{X: 4, Y: 3},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'o'}},
				[]term.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				[]term.Cell{},
				[]term.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 2},
			to:   term.Coordinates{X: 4, Y: 3},
			expected: [][]term.Cell{
				[]term.Cell{},
				[]term.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 2},
			to:   term.Coordinates{X: 5, Y: 3},
			expected: [][]term.Cell{
				[]term.Cell{},
				[]term.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 2},
			to:   term.Coordinates{X: 4, Y: 4},
			expected: [][]term.Cell{
				[]term.Cell{},
				[]term.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
	}

	for i, tcase := range testCases {
		selection := Select(buf.RawCells(), tcase.from, tcase.to)
		if !reflect.DeepEqual(selection, tcase.expected) {
			t.Errorf("tcase %d: expected %q found %q", i,
				toString(tcase.expected), toString(selection))
		}
	}
}

func TestSelectLine(t *testing.T) {
	var buf Buffer
	str := "hello\n\tworld\n\nitsme"
	buf.WriteString(str)

	testCases := []selectCase{
		{
			from: term.Coordinates{},
			to:   term.Coordinates{X: 1, Y: 0},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
			},
		},
		{
			from: term.Coordinates{X: 2, Y: 0},
			to:   term.Coordinates{X: 4, Y: 0},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
			},
		},
		{
			from: term.Coordinates{X: 2, Y: 0},
			to:   term.Coordinates{X: 7, Y: 1},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]term.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 2},
			to:   term.Coordinates{X: 2, Y: 0},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]term.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				[]term.Cell{},
			},
		},
		{
			to:   term.Coordinates{X: 2, Y: 0},
			from: term.Coordinates{X: 10, Y: 2},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]term.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				[]term.Cell{},
			},
		},
		{
			to:   term.Coordinates{X: 2, Y: 0},
			from: term.Coordinates{X: 0, Y: 10},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]term.Cell{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				[]term.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
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

	testCases := []selectCase{
		{
			from: term.Coordinates{},
			to:   term.Coordinates{X: 1, Y: 0},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'h'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 0},
			to:   term.Coordinates{X: 3, Y: 1},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}},
				[]term.Cell{{}, {}, {}, {Ch: '\t'}},
			},
		},
		{
			from: term.Coordinates{X: 3, Y: 1},
			to:   term.Coordinates{X: 0, Y: 0},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}},
				[]term.Cell{{}, {}, {}, {Ch: '\t'}},
			},
		},
		{
			from: term.Coordinates{X: 3, Y: 3},
			to:   term.Coordinates{X: 0, Y: 0},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}},
				[]term.Cell{{}, {}, {}, {Ch: '\t'}},
				[]term.Cell{},
				[]term.Cell{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}},
			},
		},
		{
			from: term.Coordinates{X: 4, Y: 0},
			to:   term.Coordinates{X: 2, Y: 6},
			expected: [][]term.Cell{
				[]term.Cell{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				[]term.Cell{{}, {Ch: '\t'}, {Ch: 'w'}},
				[]term.Cell{},
				[]term.Cell{{Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
				[]term.Cell{},
				[]term.Cell{},
				[]term.Cell{},
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
