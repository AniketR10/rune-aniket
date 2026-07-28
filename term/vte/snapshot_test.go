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
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestTerminalSnapshotStorageRoundTripTermCells(t *testing.T) {
	type doc struct {
		Snapshot Snapshot
	}

	stored := doc{Snapshot: Snapshot{
		Schema:       terminalSnapshotVersion,
		Title:        "saved",
		Width:        80,
		Height:       24,
		ScrollOffset: term.Coordinates{Y: 3, X: 1},
		Primary: ScreenSnapshot{
			Cursor: term.Coordinates{Y: 7, X: 4},
			Cells: [][]term.Cell{
				{
					{
						Fg:        term.ColorRed,
						Bg:        term.ColorBlue,
						Attrs:     term.AttrBold | term.AttrUnderline,
						Ch:        'e',
						Combining: &[]rune{'\u0301'},
						Width:     1,
						Bytes:     3,
					},
					{Ch: '界', Combining: nil, Width: 2, Bytes: 3},
				},
			},
		},
		Alternate: ScreenSnapshot{
			Cursor: term.Coordinates{Y: 1, X: 2},
			Cells: [][]term.Cell{
				{
					{Ch: 'a', Width: 1, Bytes: 1},
					{Ch: 'b', Width: 1, Bytes: 1},
				},
			},
		},
	}}

	svc := storagestub.NewInMemoryService()
	require.NoError(t, svc.Set(context.Background(), "terminal", stored))

	var actual doc
	require.NoError(t, svc.Get(context.Background(), "terminal", &actual))
	require.Equal(t, stored, actual)
}
