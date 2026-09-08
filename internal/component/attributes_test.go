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

package component

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/component/comptest"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/cell"
)

func TestDrawAttributes(t *testing.T) {
	t.Run("draws cells correctly and respecting bounds", func(t *testing.T) {
		w := term.NewStringWriter(9, 5)
		u := &component.TestComponent{Ch: 'X'}
		s := WithAttrSetter(u)

		s.Resize(8, 4)

		tests := []comptest.TestCase{
			{
				nil, `
XXXXXXXX 
XXXXXXXX 
XXXXXXXX 
XXXXXXXX 
         `,
			},
		}

		comptest.TestComponent(t, s, w, tests)
	})
}

func TestAttrSetter(t *testing.T) {
	t.Run("attr oob is ignored", func(t *testing.T) {
		w := term.NewStringWriter(2, 2)
		s := WithAttrSetter(&component.TestComponent{Ch: 'x'})
		s.SetAttrAt(term.Coordinates{X: 2, Y: 2}, term.Attributes{Fg: term.ColorRed})

		s.Resize(2, 2)

		// test that it doesn't panic
		s.Draw(w)
	})

	t.Run("happy path", func(t *testing.T) {
		w := cell.NewBufferWriter(context.Background(), 4, 4)
		s := WithAttrSetter(&component.TestComponent{Ch: 'a'})
		s.SetAttr(term.Attributes{Fg: term.ColorBlue, Bg: term.ColorNavy})
		s.SetAttrAt(term.Coordinates{X: 3, Y: 3},
			term.Attributes{
				Fg:    term.ColorRed,
				Bg:    term.ColorGreen,
				Attrs: term.AttrBold | term.AttrUnderline,
			})

		s.Resize(4, 4)
		s.Draw(w)

		for y, row := range w.RawCells() {
			for x, cell := range row {
				if y == 3 && x == 3 {
					assert.Equal(t, term.ColorGreen, cell.Bg)
					assert.Equal(t, term.ColorRed, cell.Fg)
					assert.True(t, cell.Attrs&term.AttrUnderline != 0)
					assert.True(t, cell.Attrs&term.AttrBold != 0)
				} else {
					assert.Equal(t, term.ColorNavy, cell.Bg)
					assert.Equal(t, term.ColorBlue, cell.Fg)
				}
			}
		}
	})
}
