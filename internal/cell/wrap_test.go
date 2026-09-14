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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestRowsToString(t *testing.T) {
	row := func(s string, wrapped bool) []term.Cell {
		cells := term.StringToCells(s)[0]
		if wrapped {
			cells[len(cells)-1].Bytes = WrapMarker
		}
		return cells
	}

	cases := []struct {
		desc string
		rows [][]term.Cell
		want string
	}{
		{"empty", nil, ""},
		{"single row", [][]term.Cell{row("abc", false)}, "abc"},
		{
			desc: "unmarked rows keep their newline",
			rows: [][]term.Cell{row("abc", false), row("def", false)},
			want: "abc\ndef",
		},
		{
			desc: "marked row joins its continuation",
			rows: [][]term.Cell{row("abc", true), row("def", false)},
			want: "abcdef",
		},
		{
			desc: "consecutive marked rows join into one line",
			rows: [][]term.Cell{
				row("abc", true), row("def", true), row("gh", false),
			},
			want: "abcdefgh",
		},
		{
			desc: "newline resumes after the continuation row",
			rows: [][]term.Cell{
				row("abc", true), row("def", false), row("gh", false),
			},
			want: "abcdef\ngh",
		},
		{
			desc: "trailing empty row still yields a newline",
			rows: [][]term.Cell{row("abc", true), {}},
			want: "abc\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.want, RowsToString(tc.rows))
		})
	}
}
