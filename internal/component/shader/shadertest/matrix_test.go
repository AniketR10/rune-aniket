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

package shadertest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestMakeCellMatrix(t *testing.T) {
	tsuite := []struct {
		name   string
		rows   int
		cols   int
		panics bool
	}{
		{
			name: "creates a list of 'rows' lists with 'cols' cells each of them",
			rows: 4,
			cols: 7,
		},
		{
			name: "zero cells",
			rows: 0,
			cols: 0,
		},
		{
			name:   "negative dimensions",
			rows:   -4,
			cols:   -7,
			panics: true,
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			if tcase.panics {
				assert.Panics(t, func() {
					MakeCellMatrix(tcase.cols, tcase.rows)
				})
				return
			}

			cells := MakeCellMatrix(tcase.cols, tcase.rows)
			assert.Len(t, cells, tcase.rows)

			for y := 0; y < tcase.rows; y++ {
				assert.Len(t, cells[y], tcase.cols)
				for x := 0; x < tcase.cols; x++ {
					assert.Equal(t, term.Cell{}, cells[y][x])
				}
			}
		})
	}
}
