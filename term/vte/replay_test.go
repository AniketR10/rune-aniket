// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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
