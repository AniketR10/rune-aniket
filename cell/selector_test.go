package cell

import (
	"reflect"
	"testing"

	"github.com/ernestrc/go-tui/term"
)

func TestSelect(t *testing.T) {
	str := `hello
	world

itsme`
	buf := newBufferWithContent(t, str)

	testCases := []selectCase{
		{
			from: term.Coordinates{},
			to:   term.Coordinates{X: 1, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{X: 2, Y: 0},
			to:   term.Coordinates{X: 1, Y: 1},
			expected: [][]term.Cell{
				{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{{}, {}},
			},
		},
		{
			from: term.Coordinates{X: 2, Y: 0},
			to:   term.Coordinates{X: 1, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'e'}, {Ch: 'l'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 1},
			to:   term.Coordinates{X: 8, Y: 1},
			expected: [][]term.Cell{
				{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
			},
		},
		{
			from: term.Coordinates{X: 2, Y: 0},
			to:   term.Coordinates{X: 4, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
			},
		},
		{
			from: term.Coordinates{X: 2, Y: 0},
			to:   term.Coordinates{X: 7, Y: 1},
			expected: [][]term.Cell{
				{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}},
			},
		},
		{
			from: term.Coordinates{X: 7, Y: 1},
			to:   term.Coordinates{X: 2, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}},
			},
		},
		{
			from: term.Coordinates{X: 4, Y: 0},
			to:   term.Coordinates{X: 4, Y: 3},
			expected: [][]term.Cell{
				{{Ch: 'o'}},
				{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				{},
				{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 2},
			to:   term.Coordinates{X: 4, Y: 3},
			expected: [][]term.Cell{
				{},
				{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 2},
			to:   term.Coordinates{X: 5, Y: 3},
			expected: [][]term.Cell{
				{},
				{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 2},
			to:   term.Coordinates{X: 4, Y: 4},
			expected: [][]term.Cell{
				{},
				{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 0},
			to:   term.Coordinates{X: 5, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 3},
			to:   term.Coordinates{X: 5, Y: 3},
			expected: [][]term.Cell{
				{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
	}

	for i, tcase := range testCases {
		selector := selector{reader: buf.reader}
		selection := selector.selectCells(tcase.from, tcase.to)
		if !reflect.DeepEqual(selection, tcase.expected) {
			t.Errorf("tcase %d: expected %q found %q", i,
				CellsToString(tcase.expected), CellsToString(selection))
		}
	}
}

func TestSelectLine(t *testing.T) {
	str := "hello\n\tworld\n\nitsme"
	buf := newBufferWithContent(t, str)

	testCases := []selectCase{
		{
			from: term.Coordinates{},
			to:   term.Coordinates{X: 1, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{},
			},
		},
		{
			from: term.Coordinates{X: 2, Y: 0},
			to:   term.Coordinates{X: 4, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{},
			},
		},
		{
			from: term.Coordinates{X: 2, Y: 0},
			to:   term.Coordinates{X: 7, Y: 1},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				{},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 2},
			to:   term.Coordinates{X: 2, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				{},
			},
		},
		{
			to:   term.Coordinates{X: 2, Y: 0},
			from: term.Coordinates{X: 10, Y: 2},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				{},
			},
		},
		{
			to:   term.Coordinates{X: 2, Y: 0},
			from: term.Coordinates{X: 0, Y: 10},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				{},
				{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 1},
			to:   term.Coordinates{X: 0, Y: 1},
			expected: [][]term.Cell{
				{{}, {}, {}, {Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				{},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 3},
			to:   term.Coordinates{X: 0, Y: 3},
			expected: [][]term.Cell{
				{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
	}

	for _, tcase := range testCases {
		selector := selector{reader: buf.reader}
		selection := selector.selectLine(tcase.from, tcase.to)
		if CellsToString(selection) != CellsToString(tcase.expected) {
			t.Errorf("expected %q found %q", CellsToString(tcase.expected), CellsToString(selection))
		}
	}
}

func TestSelectBlock(t *testing.T) {
	str := "hello\n\tworld\n\nitsme\n\n\nhi"
	buf := newBufferWithContent(t, str)

	testCases := []selectCase{
		{
			from: term.Coordinates{},
			to:   term.Coordinates{X: 1, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 0},
			to:   term.Coordinates{X: 3, Y: 1},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}},
				{{}, {}, {}, {Ch: '\t'}},
			},
		},
		{
			from: term.Coordinates{X: 3, Y: 1},
			to:   term.Coordinates{X: 0, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}},
				{{}, {}, {}, {Ch: '\t'}},
			},
		},
		{
			from: term.Coordinates{X: 3, Y: 3},
			to:   term.Coordinates{X: 0, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}},
				{{}, {}, {}, {Ch: '\t'}},
				{},
				{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}},
			},
		},
		{
			from: term.Coordinates{X: 4, Y: 0},
			to:   term.Coordinates{X: 2, Y: 6},
			expected: [][]term.Cell{
				{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{{}, {Ch: '\t'}, {Ch: 'w'}},
				{},
				{{Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
				{},
				{},
				{},
			},
		},
	}

	for _, tcase := range testCases {
		selector := selector{reader: buf.reader}
		selection := selector.selectBlock(tcase.from, tcase.to)
		if !reflect.DeepEqual(selection, tcase.expected) {
			t.Errorf("expected %q found %q", CellsToString(tcase.expected), CellsToString(selection))
		}
	}
}
