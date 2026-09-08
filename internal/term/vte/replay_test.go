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

package vte

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestReplay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		width       int
		height      int
		data        string
		wantRow0    string // first row after replay, trimmed of trailing spaces
		wantRow1    string
		wantCursor  term.Coordinates
		wantInitErr bool
	}{
		{
			name:       "plain hello then cup",
			width:      10,
			height:     3,
			data:       "hello\x1b[2;1Hworld",
			wantRow0:   "hello",
			wantRow1:   "world",
			wantCursor: term.Coordinates{X: 5, Y: 1},
		},
		{
			name:       "carriage return then linefeed places cursor",
			width:      6,
			height:     2,
			data:       "ab\r\ncd",
			wantRow0:   "ab",
			wantRow1:   "cd",
			wantCursor: term.Coordinates{X: 2, Y: 1},
		},
		{
			name:        "zero dimensions rejected",
			width:       0,
			height:      24,
			data:        "",
			wantInitErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf, cur, err := Replay(tt.width, tt.height, []byte(tt.data))
			if tt.wantInitErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, buf)

			cells := buf.RawCells()
			require.GreaterOrEqual(t, len(cells), 2, "expected at least two rows")
			assert.Equal(t, tt.wantRow0, trimRight(cells[0]))
			assert.Equal(t, tt.wantRow1, trimRight(cells[1]))
			assert.Equal(t, tt.wantCursor, cur)
		})
	}
}

// trimRight renders the cells of a row as a string with trailing
// spaces removed. We use it because Replay pads rows with the default
// char (space) to the buffer width.
func trimRight(row []term.Cell) string {
	end := len(row)
	for end > 0 && (row[end-1].Ch == ' ' || row[end-1].Ch == 0) {
		end--
	}
	runes := make([]rune, 0, end)
	for i := 0; i < end; i++ {
		runes = append(runes, row[i].Ch)
	}
	return string(runes)
}
