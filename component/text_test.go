// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestWriteText(t *testing.T) {
	tests := []struct {
		name      string
		text      string
		x         int
		maxX      int
		wantNextX int
		wantCells map[int]term.Cell
	}{
		{
			name:      "ascii",
			text:      "abc",
			maxX:      10,
			wantNextX: 3,
			wantCells: map[int]term.Cell{
				0: {Ch: 'a', Width: 1},
				1: {Ch: 'b', Width: 1},
				2: {Ch: 'c', Width: 1},
			},
		},
		{
			name:      "empty string writes nothing",
			text:      "",
			maxX:      10,
			wantNextX: 0,
			wantCells: map[int]term.Cell{0: {}},
		},
		{
			name:      "nul byte occupies one cell",
			text:      "a\x00b",
			maxX:      10,
			wantNextX: 3,
			wantCells: map[int]term.Cell{
				0: {Ch: 'a', Width: 1},
				1: {Ch: 0, Width: 1},
				2: {Ch: 'b', Width: 1},
			},
		},
		{
			name:      "tab occupies one cell",
			text:      "\ta",
			maxX:      10,
			wantNextX: 2,
			wantCells: map[int]term.Cell{
				0: {Ch: '\t', Width: 1},
				1: {Ch: 'a', Width: 1},
			},
		},
		{
			name:      "bell control occupies one cell",
			text:      "\aa",
			maxX:      10,
			wantNextX: 2,
			wantCells: map[int]term.Cell{
				0: {Ch: '\a', Width: 1},
				1: {Ch: 'a', Width: 1},
			},
		},
		{
			name:      "del control occupies one cell",
			text:      "\x7fa",
			maxX:      10,
			wantNextX: 2,
			wantCells: map[int]term.Cell{
				0: {Ch: '\x7f', Width: 1},
				1: {Ch: 'a', Width: 1},
			},
		},
		{
			name:      "spaces occupy one cell each",
			text:      "a  b",
			maxX:      10,
			wantNextX: 4,
			wantCells: map[int]term.Cell{
				0: {Ch: 'a', Width: 1},
				1: {Ch: ' ', Width: 1},
				2: {Ch: ' ', Width: 1},
				3: {Ch: 'b', Width: 1},
			},
		},
		{
			name:      "lone combining mark occupies one cell",
			text:      "\u0301a",
			maxX:      10,
			wantNextX: 2,
			wantCells: map[int]term.Cell{
				0: {Ch: '\u0301', Width: 1},
				1: {Ch: 'a', Width: 1},
			},
		},
		{
			name:      "zero width space occupies one cell",
			text:      "a\u200bb",
			maxX:      10,
			wantNextX: 3,
			wantCells: map[int]term.Cell{
				0: {Ch: 'a', Width: 1},
				1: {Ch: '\u200b', Width: 1},
				2: {Ch: 'b', Width: 1},
			},
		},
		{
			name:      "accented rune is one narrow cell",
			text:      "héllo",
			maxX:      10,
			wantNextX: 5,
			wantCells: map[int]term.Cell{
				0: {Ch: 'h', Width: 1},
				1: {Ch: 'é', Width: 1},
				2: {Ch: 'l', Width: 1},
			},
		},
		{
			name:      "fullwidth latin takes two columns",
			text:      "ｗa",
			maxX:      10,
			wantNextX: 3,
			wantCells: map[int]term.Cell{
				0: {Ch: 'ｗ', Width: 2},
				1: {},
				2: {Ch: 'a', Width: 1},
			},
		},
		{
			name:      "halfwidth katakana takes one column",
			text:      "ﾜa",
			maxX:      10,
			wantNextX: 2,
			wantCells: map[int]term.Cell{
				0: {Ch: 'ﾜ', Width: 1},
				1: {Ch: 'a', Width: 1},
			},
		},
		{
			name:      "nerd font icon takes two columns",
			text:      "󰗠a",
			maxX:      10,
			wantNextX: 3,
			wantCells: map[int]term.Cell{
				0: {Ch: '󰗠', Width: 2},
				1: {},
				2: {Ch: 'a', Width: 1},
			},
		},
		{
			name:      "astral narrow rune takes one column",
			text:      "𝄞a",
			maxX:      10,
			wantNextX: 2,
			wantCells: map[int]term.Cell{
				0: {Ch: '𝄞', Width: 1},
				1: {Ch: 'a', Width: 1},
			},
		},
		{
			name:      "circled digit narrow circled number wide",
			text:      "①㉑",
			maxX:      10,
			wantNextX: 3,
			wantCells: map[int]term.Cell{
				0: {Ch: '①', Width: 1},
				1: {Ch: '㉑', Width: 2},
				2: {},
			},
		},
		{
			name:      "emoji takes two columns and skips the continuation",
			text:      "a🚀b",
			maxX:      10,
			wantNextX: 4,
			wantCells: map[int]term.Cell{
				0: {Ch: 'a', Width: 1},
				1: {Ch: '🚀', Width: 2},
				2: {},
				3: {Ch: 'b', Width: 1},
			},
		},
		{
			name:      "cjk",
			text:      "日本",
			maxX:      10,
			wantNextX: 4,
			wantCells: map[int]term.Cell{
				0: {Ch: '日', Width: 2},
				1: {},
				2: {Ch: '本', Width: 2},
				3: {},
			},
		},
		{
			name:      "ascii to non-ascii transitions",
			text:      "aéa中b",
			maxX:      10,
			wantNextX: 6,
			wantCells: map[int]term.Cell{
				0: {Ch: 'a', Width: 1},
				1: {Ch: 'é', Width: 1},
				2: {Ch: 'a', Width: 1},
				3: {Ch: '中', Width: 2},
				4: {},
				5: {Ch: 'b', Width: 1},
			},
		},
		{
			name:      "wide cluster does not straddle maxX",
			text:      "a🚀",
			maxX:      2,
			wantNextX: 1,
			wantCells: map[int]term.Cell{
				0: {Ch: 'a', Width: 1},
				1: {},
			},
		},
		{
			name:      "wide cluster exactly fits maxX",
			text:      "🚀",
			maxX:      2,
			wantNextX: 2,
			wantCells: map[int]term.Cell{
				0: {Ch: '🚀', Width: 2},
				1: {},
			},
		},
		{
			name:      "ascii exactly fits maxX",
			text:      "ab",
			maxX:      2,
			wantNextX: 2,
			wantCells: map[int]term.Cell{
				0: {Ch: 'a', Width: 1},
				1: {Ch: 'b', Width: 1},
			},
		},
		{
			name:      "x already at maxX writes nothing",
			text:      "abc",
			x:         5,
			maxX:      5,
			wantNextX: 5,
			wantCells: map[int]term.Cell{5: {}},
		},
		{
			name:      "zero maxX writes nothing",
			text:      "abc",
			maxX:      0,
			wantNextX: 0,
			wantCells: map[int]term.Cell{0: {}},
		},
		{
			name:      "stops entirely at first cluster that does not fit",
			text:      "🚀a",
			x:         1,
			maxX:      2,
			wantNextX: 1,
			wantCells: map[int]term.Cell{
				1: {},
			},
		},
		{
			name:      "starts at the given column",
			text:      "hi",
			x:         3,
			maxX:      10,
			wantNextX: 5,
			wantCells: map[int]term.Cell{
				3: {Ch: 'h', Width: 1},
				4: {Ch: 'i', Width: 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := term.NewStringWriter(10, 1)
			next := WriteText(w, tt.x, 0, tt.maxX, tt.text, term.Attributes{})
			assert.Equal(t, tt.wantNextX, next)
			cells := w.Cells()
			for x, want := range tt.wantCells {
				assert.Equal(t, want.Ch, cells[x].Ch, "cell %d rune", x)
				assert.Equal(t, want.Width, cells[x].Width, "cell %d width", x)
			}
		})
	}
}

func TestWriteTextCombiningClusters(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		wantCh        rune
		wantWidth     uint8
		wantCombining []rune
	}{
		{
			name:          "zwj family stays in one cell",
			text:          "👨‍👩‍👧",
			wantCh:        '👨',
			wantWidth:     2,
			wantCombining: []rune{'\u200d', '👩', '\u200d', '👧'},
		},
		{
			name:          "variation selector stays in one cell",
			text:          "❤️",
			wantCh:        '❤',
			wantWidth:     2,
			wantCombining: []rune{'\ufe0f'},
		},
		{
			name:          "combining mark after ascii stays in one cell",
			text:          "e\u0301",
			wantCh:        'e',
			wantWidth:     1,
			wantCombining: []rune{'\u0301'},
		},
		{
			name:          "regional indicator flag stays in one cell",
			text:          "🇺🇸",
			wantCh:        '🇺',
			wantWidth:     2,
			wantCombining: []rune{'🇸'},
		},
		{
			name:          "skin tone modifier stays in one cell",
			text:          "👍🏽",
			wantCh:        '👍',
			wantWidth:     2,
			wantCombining: []rune{'\U0001f3fd'},
		},
		{
			name:          "zwj after ascii attaches to the ascii cell",
			text:          "a\u200d",
			wantCh:        'a',
			wantWidth:     1,
			wantCombining: []rune{'\u200d'},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := term.NewStringWriter(10, 1)
			next := WriteText(w, 0, 0, 10, tt.text, term.Attributes{})
			assert.Equal(t, int(tt.wantWidth), next)
			cells := w.Cells()
			assert.Equal(t, tt.wantCh, cells[0].Ch)
			assert.Equal(t, tt.wantWidth, cells[0].Width)
			assert.Equal(t, tt.wantCombining, cells[0].CombiningRunes())
			assert.Equal(t, rune(0), cells[1].Ch)
		})
	}
}
