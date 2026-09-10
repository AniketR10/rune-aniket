// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package cell

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestSelect(t *testing.T) {
	str := `hello
	world

itsme`
	buf := newBufferWithContent(t, str)

	testCases := []selectCase{
		{
			from: term.Coordinates{},
			to:   term.Coordinates{X: 2, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{X: 2, Y: 0},
			to:   term.Coordinates{X: 2, Y: 1},
			expected: [][]term.Cell{
				{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{{Ch: '\t'}, {Ch: 'w'}},
			},
		},
		{
			from: term.Coordinates{X: 3, Y: 0},
			to:   term.Coordinates{X: 1, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'e'}, {Ch: 'l'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 1},
			to:   term.Coordinates{X: 9, Y: 1},
			expected: [][]term.Cell{
				{{Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
			},
		},
		{
			from: term.Coordinates{X: 2, Y: 0},
			to:   term.Coordinates{X: 5, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
			},
		},
		{
			from: term.Coordinates{X: 2, Y: 0},
			to:   term.Coordinates{X: 8, Y: 1},
			expected: [][]term.Cell{
				{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{{Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
			},
		},
		{
			from: term.Coordinates{X: 8, Y: 1},
			to:   term.Coordinates{X: 2, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{{Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
			},
		},
		{
			from: term.Coordinates{X: 4, Y: 0},
			to:   term.Coordinates{X: 5, Y: 3},
			expected: [][]term.Cell{
				{{Ch: 'o'}},
				{{Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
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
			to:   term.Coordinates{X: 6, Y: 3},
			expected: [][]term.Cell{
				{},
				{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 2},
			to:   term.Coordinates{X: 5, Y: 4},
			expected: [][]term.Cell{
				{},
				{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{},
			to:   term.Coordinates{Y: 1},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 3},
			to:   term.Coordinates{X: 6, Y: 3},
			expected: [][]term.Cell{
				{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
	}

	for i, tcase := range testCases {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			selector := selector{view: buf.view}
			selection, coords := selector.selectCells(tcase.from, tcase.to)
			// we do not care about width; makes defining tests easier
			for y, row := range selection {
				for x := range row {
					selection[y][x].Width = 0
				}
			}
			require.Equal(t, term.CellsToString(tcase.expected), term.CellsToString(selection))
			assertReturnedCoordinatesSelectSame(t, selector, coords, tcase.expected)
		})
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
			},
		},
		{
			from: term.Coordinates{X: 2, Y: 0},
			to:   term.Coordinates{X: 4, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
			},
		},
		{
			from: term.Coordinates{X: 2, Y: 0},
			to:   term.Coordinates{X: 7, Y: 1},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{{Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 2},
			to:   term.Coordinates{X: 2, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{{Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				{},
			},
		},
		{
			to:   term.Coordinates{X: 2, Y: 0},
			from: term.Coordinates{X: 10, Y: 2},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{{Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				{},
			},
		},
		{
			to:   term.Coordinates{X: 2, Y: 0},
			from: term.Coordinates{X: 0, Y: 10},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{{Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
				{},
				{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 1},
			to:   term.Coordinates{X: 0, Y: 1},
			expected: [][]term.Cell{
				{{Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}, {Ch: 'd'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 3},
			to:   term.Coordinates{X: 0, Y: 3},
			expected: [][]term.Cell{
				{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 2},
			to:   term.Coordinates{X: 0, Y: 2},
			expected: [][]term.Cell{
				{},
			},
		},
	}

	for i, tcase := range testCases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			selector := selector{view: buf.view}
			selection, coords := selector.selectLine(tcase.from, tcase.to)
			// we do not care about width; makes defining tests easier
			for y, row := range selection {
				for x := range row {
					selection[y][x].Width = 0
				}
			}
			require.Equal(t, term.CellsToString(tcase.expected), term.CellsToString(selection))
			assertReturnedCoordinatesSelectSame(t, selector, coords, tcase.expected)
		})
	}
}

func TestSelectBlock(t *testing.T) {
	str := "hello\n\tworld\n\nitsme\n\n\nhi"
	buf := newBufferWithContent(t, str)

	testCases := []selectCase{
		{
			from: term.Coordinates{},
			to:   term.Coordinates{X: 2, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}},
			},
		},
		{
			from: term.Coordinates{X: 0, Y: 0},
			to:   term.Coordinates{X: 4, Y: 1},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}},
				{{Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}},
			},
		},
		{
			from: term.Coordinates{X: 4, Y: 1},
			to:   term.Coordinates{X: 0, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}},
				{{Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}},
			},
		},
		{
			from: term.Coordinates{X: 4, Y: 3},
			to:   term.Coordinates{X: 0, Y: 0},
			expected: [][]term.Cell{
				{{Ch: 'h'}, {Ch: 'e'}, {Ch: 'l'}, {Ch: 'l'}},
				{{Ch: '\t'}, {Ch: 'w'}, {Ch: 'o'}, {Ch: 'r'}},
				{},
				{{Ch: 'i'}, {Ch: 't'}, {Ch: 's'}, {Ch: 'm'}},
			},
		},
		{
			from: term.Coordinates{X: 5, Y: 0},
			to:   term.Coordinates{X: 2, Y: 6},
			expected: [][]term.Cell{
				{{Ch: 'l'}, {Ch: 'l'}, {Ch: 'o'}},
				{{Ch: 'o'}, {Ch: 'r'}, {Ch: 'l'}},
				{},
				{{Ch: 's'}, {Ch: 'm'}, {Ch: 'e'}},
				{},
				{},
				{},
			},
		},
	}

	for i, tcase := range testCases {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			selector := selector{view: buf.view}
			selection, coords := selector.selectBlock(tcase.from, tcase.to)
			// we do not care about width; makes defining tests easier
			for y, row := range selection {
				for x := range row {
					selection[y][x].Width = 0
				}
			}
			require.Equal(t, term.CellsToString(tcase.expected), term.CellsToString(selection), i)
			assertReturnedCoordinatesSelectSame(t, selector, coords, tcase.expected)
		})
	}
}

func assertReturnedCoordinatesSelectSame(
	t *testing.T, selector selector,
	coords []Selection, expected [][]term.Cell,
) {
	t.Helper()
	var selectedSelection [][]term.Cell
	for _, coords := range coords {
		cells, actualCoords := selector.selectCells(coords.From, coords.To)
		// it shouldn't break apart further
		require.Len(t, actualCoords, 1)
		assert.Equal(t, coords, actualCoords[0])
		selectedSelection = append(selectedSelection, cells...)
	}
	assert.Equal(t, term.CellsToString(expected), term.CellsToString(selectedSelection))
}
