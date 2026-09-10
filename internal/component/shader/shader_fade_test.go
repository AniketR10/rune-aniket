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

package shader

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestFade(t *testing.T) {
	suite := []struct {
		frame        int
		total        int
		defaultAttrs term.Attributes
		in           [][]term.Cell
		wantOut      [][]term.Cell
	}{
		{},
		{frame: 0, total: 9, in: [][]term.Cell{}, wantOut: [][]term.Cell{}},
		{
			frame: 0,
			total: 9,
			in: [][]term.Cell{{
				term.NewCell(0, 0, term.Attributes{Fg: 0, Bg: term.ColorBlack}),
			}},
			wantOut: [][]term.Cell{{
				term.NewCell(0, 0, term.Attributes{Fg: term.ColorBlack, Bg: term.ColorBlack}),
			}},
		},
		{
			frame: 10,
			total: 9,
			in: [][]term.Cell{{
				term.NewCell(0, 0, term.Attributes{Fg: 0, Bg: term.ColorBlack}),
			}},
			wantOut: [][]term.Cell{{
				term.NewCell(0, 0, term.Attributes{Fg: 0, Bg: term.ColorBlack}),
			}},
		},
		{
			frame: 4,
			total: 9,
			in: [][]term.Cell{{
				term.NewCell(0, 0, term.Attributes{Fg: term.NewRGBColor(10, 10, 10), Bg: term.ColorBlack}),
			}},
			wantOut: [][]term.Cell{{
				term.NewCell(0, 0, term.Attributes{Fg: term.NewRGBColor(4, 4, 4), Bg: term.ColorBlack}),
			}},
		},
		{
			frame: 4,
			total: 9,
			in: [][]term.Cell{{
				term.NewCell(0, 0, term.Attributes{Fg: term.NewRGBColor(10, 10, 10), Bg: 0}),
			}},
			// Bg is ColorDefault and defaultAttrs.Bg is also unset, so the
			// blend has no resolvable starting point and Fg ends up
			// resolving to ColorDefault.
			wantOut: [][]term.Cell{{
				term.NewCell(0, 0, term.Attributes{Fg: term.ColorDefault, Bg: 0}),
			}},
		},
		{
			frame:        4,
			total:        9,
			defaultAttrs: term.Attributes{Fg: term.NewRGBColor(10, 0, 0), Bg: term.NewRGBColor(0, 0, 10)},
			in: [][]term.Cell{{
				term.NewCell(0, 0, term.Attributes{Fg: term.ColorDefault, Bg: term.ColorDefault}),
			}},
			wantOut: [][]term.Cell{{
				term.NewCell(0, 0, term.Attributes{Fg: term.NewRGBColor(4, 0, 5), Bg: term.ColorDefault}),
			}},
		},
	}

	for _, test := range suite {
		Fade(test.defaultAttrs).Shade(test.frame, test.total, test.in)
		assert.Equal(t, test.wantOut, test.in)
	}
}
