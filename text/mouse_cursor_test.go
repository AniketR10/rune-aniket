// Copyright (C) 2017-2026 Unstable Build, LLC
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

package text

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/cell"
	"unstable.build/rune/component"
)

// TestMouseDelegateSelectWordAtInvertedScrollAboveBuffer reproduces
// crash report 787830382: "runtime error: index out of range [-3]".
// vte terminals run with Scroll.InvertOffset=true, which makes
// WindowToScrollCoordinates legitimately return a negative Y for
// window positions that sit above the bottom-anchored buffer content
// (tall window, sparse buffer). The mouse delegate must treat those
// out-of-buffer positions as "nothing to select" rather than handing
// the negative coordinate to Scroll.WordAt, which would panic indexing
// rawCells.cells[-3].
func TestMouseDelegateSelectWordAtInvertedScrollAboveBuffer(t *testing.T) {
	scroll := component.NewScroll(cell.NewBuffer())
	scroll.InvertOffset = true
	_, _ = scroll.Buffer().ReadFrom(strings.NewReader("hello\n"))
	// Window is much taller than the populated buffer rows, so
	// convertedOffset().Y is negative and clicks near the top of the
	// window translate to negative scroll coordinates.
	scroll.Resize(80, 24)

	cur := NewCursor(scroll, nil)
	delegate := CursorMouseDelegate(cur)

	assert.NotPanics(t, func() {
		delegate.SelectWordAt(term.Coordinates{X: 0, Y: 0})
	})
	_, hasSelection := cur.SelectionMode()
	assert.False(t, hasSelection,
		"clicks above the buffer must not start a selection")
}

// TestScrollWordAtOutOfBounds locks in the documented "empty string if
// token at the given position is not a word" contract for inputs that
// fall outside the buffer. This is a defense-in-depth guard: every
// other call site that routes window coordinates through
// WindowToScrollCoordinates and into WordAt is protected here.
func TestScrollWordAtOutOfBounds(t *testing.T) {
	scroll := component.NewScroll(cell.NewBuffer())
	_, _ = scroll.Buffer().ReadFrom(strings.NewReader("hello world\n"))
	scroll.Resize(80, 24)

	cases := []struct {
		name string
		at   term.Coordinates
	}{
		{"negative Y", term.Coordinates{X: 0, Y: -3}},
		{"negative X", term.Coordinates{X: -1, Y: 0}},
		{"negative both", term.Coordinates{X: -2, Y: -5}},
		{"Y past last row", term.Coordinates{X: 0, Y: 1 << 20}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				_, _, got := scroll.WordAt(tc.at)
				assert.Equal(t, "", got)
			})
		})
	}
}

// TestMouseDelegateSelectWordAtWiderThanWindow double-clicks a word
// longer than the viewport, so anchoring the selection at the word
// start scrolls the window horizontally. The selection end must still
// land on the word end rather than follow the shifted offset.
func TestMouseDelegateSelectWordAtWiderThanWindow(t *testing.T) {
	const content = "aa bbbbbbbbbbbbbbbbbb"
	tests := []struct {
		name    string
		cursorX int
		clickX  int
	}{
		{"viewport scrolled to end of line", len(content), 4},
		{"viewport scrolled mid word", 15, 2},
		{"viewport at start of line", 0, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scroll := component.NewScroll(cell.NewBuffer())
			_, _ = scroll.Buffer().ReadFrom(strings.NewReader(content))
			scroll.Resize(8, 1)

			cur := NewCursor(scroll, nil)
			delegate := CursorMouseDelegate(cur)
			cur.MoveToScroll(term.Coordinates{X: tt.cursorX})

			delegate.SelectWordAt(term.Coordinates{X: tt.clickX})

			assert.Equal(t, "bbbbbbbbbbbbbbbbbb", cur.Selection())
			assert.Equal(t, term.Coordinates{X: len(content)}, cur.CursorAtScroll())
		})
	}
}
