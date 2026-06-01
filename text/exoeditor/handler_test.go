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

package exoeditor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/text"
)

func TestPickLocation(t *testing.T) {
	t.Parallel()

	locs := []textapi.Location{
		{From: term.Coordinates{X: 0, Y: 1}},
		{From: term.Coordinates{X: 4, Y: 3}},
		{From: term.Coordinates{X: 2, Y: 5}},
	}

	cases := []struct {
		name    string
		cursor  term.Coordinates
		forward bool
		want    term.Coordinates
	}{
		{
			name:    "next from start jumps to first",
			cursor:  term.Coordinates{X: 0, Y: 0},
			forward: true,
			want:    locs[0].From,
		},
		{
			name:    "next from middle jumps to next",
			cursor:  term.Coordinates{X: 0, Y: 3},
			forward: true,
			want:    locs[1].From,
		},
		{
			name:    "next past last wraps to first",
			cursor:  term.Coordinates{X: 9, Y: 9},
			forward: true,
			want:    locs[0].From,
		},
		{
			name:    "prev from end jumps to last",
			cursor:  term.Coordinates{X: 9, Y: 9},
			forward: false,
			want:    locs[2].From,
		},
		{
			name:    "prev from middle jumps to prev",
			cursor:  term.Coordinates{X: 9, Y: 3},
			forward: false,
			want:    locs[1].From,
		},
		{
			name:    "prev before first wraps to last",
			cursor:  term.Coordinates{X: 0, Y: 0},
			forward: false,
			want:    locs[2].From,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := pickLocation(
				text.LocationSlice(locs), tc.cursor, tc.forward)
			assert.True(t, ok)
			assert.Equal(t, tc.want, got.From)
		})
	}
}

func TestPickLocationEmptyListReturnsFalse(t *testing.T) {
	t.Parallel()
	_, ok := pickLocation(
		text.LocationSlice(nil), term.Coordinates{}, true)
	assert.False(t, ok)
}

// TestMoveToLocationReturnsFalseForUnknownList guards the public
// MoveTo{Next,Prev}Location contract for IDs that were never set via
// SetLocationList.
func TestMoveToLocationReturnsFalseForUnknownList(t *testing.T) {
	t.Parallel()
	h := &editorHandler{locations: text.NewLocationStore()}
	assert.False(t, h.MoveToNextLocation("missing"))
	assert.False(t, h.MoveToPrevLocation("missing"))
}
