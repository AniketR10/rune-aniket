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

package vteprobe

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExpandTabs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		line    string
		tabstop int
		want    string
	}{
		{"no tabs", "hello world", 4, "hello world"},
		{"single leading tab tab=4", "\tfoo", 4, "    foo"},
		{"two leading tabs tab=4", "\t\tfoo", 4, "        foo"},
		{"mid-line tab tab=4", "ab\tcd", 4, "ab  cd"},
		{"tabstop=2", "\tfoo", 2, "  foo"},
		{"tabstop=8", "\tfoo", 8, "        foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, expandTabs(tt.line, tt.tabstop))
		})
	}
}

func TestVisualToRawCol(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		line       string
		runeOffset int
		tabstop    int
		want       int
	}{
		{"no tabs offset 0", "abc", 0, 4, 1},
		{"no tabs offset 1", "abc", 1, 4, 2},
		{"no tabs offset 2", "abc", 2, 4, 3},
		{"single tab offset 4 ts4 -> col 2", "\tabc", 4, 4, 2},
		{"single tab offset 5 ts4 -> col 2 (a)", "\tabc", 4, 4, 2},
		{"tab tab offset 8 ts4 -> col 3 (after both tabs)", "\t\tabc", 8, 4, 3},
		{"past end", "abc", 100, 4, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := visualToRawColCells(drawRow(tt.line, nil), tt.runeOffset, tt.tabstop)
			assert.Equal(t, tt.want, got)
		})
	}
}
