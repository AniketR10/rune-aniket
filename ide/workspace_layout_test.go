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

package ide

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	tcomponent "unstable.build/go-tui/component"
	"unstable.build/go-tui/term/vte"
)

func TestWorkspaceLayoutStorage(t *testing.T) {
	workspaceURI, err := workspaceapi.ParseURI("memory:///workspace")
	require.NoError(t, err)
	otherWorkspaceURI, err := workspaceapi.ParseURI("memory:///other-workspace")
	require.NoError(t, err)

	complexLayout := tcomponent.TileLayout{
		Split: tcomponent.SplitOrientationVertical,
		Children: []tcomponent.TileLayout{
			{WindowID: 1, Children: []tcomponent.TileLayout{}},
			{
				Split: tcomponent.SplitOrientationHorizontal,
				Children: []tcomponent.TileLayout{
					{WindowID: 2, Children: []tcomponent.TileLayout{}},
					{WindowID: 3, Children: []tcomponent.TileLayout{}},
				},
			},
		},
	}

	tests := []struct {
		name string
		run  func(*testing.T)
	}{
		{
			name: "missing layout returns false",
			run: func(t *testing.T) {
				store := storagestub.NewInMemoryService()
				loaded, ok, err := loadWorkspaceLayout(context.Background(), store, workspaceURI)
				require.NoError(t, err)
				require.False(t, ok)
				require.Equal(t, tcomponent.TileLayout{}, loaded)
			},
		},
		{
			name: "single leaf round trip",
			run: func(t *testing.T) {
				store := storagestub.NewInMemoryService()
				layout := tcomponent.TileLayout{WindowID: 7, Children: []tcomponent.TileLayout{}}
				require.NoError(t, saveWorkspaceLayout(context.Background(), store, workspaceURI, layout))

				loaded, ok, err := loadWorkspaceLayout(context.Background(), store, workspaceURI)
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, layout, loaded)
			},
		},
		{
			name: "nested layout round trip",
			run: func(t *testing.T) {
				store := storagestub.NewInMemoryService()
				require.NoError(t, saveWorkspaceLayout(context.Background(), store, workspaceURI, complexLayout))

				loaded, ok, err := loadWorkspaceLayout(context.Background(), store, workspaceURI)
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, complexLayout, loaded)
			},
		},
		{
			name: "workspace keys are isolated",
			run: func(t *testing.T) {
				store := storagestub.NewInMemoryService()
				otherLayout := tcomponent.TileLayout{WindowID: 99, Children: []tcomponent.TileLayout{}}
				require.NoError(t, saveWorkspaceLayout(context.Background(), store, workspaceURI, complexLayout))
				require.NoError(t, saveWorkspaceLayout(context.Background(), store, otherWorkspaceURI, otherLayout))

				loaded, ok, err := loadWorkspaceLayout(context.Background(), store, workspaceURI)
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, complexLayout, loaded)

				loaded, ok, err = loadWorkspaceLayout(context.Background(), store, otherWorkspaceURI)
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, otherLayout, loaded)
			},
		},
		{
			name: "clear removes only requested workspace layout",
			run: func(t *testing.T) {
				store := storagestub.NewInMemoryService()
				otherLayout := tcomponent.TileLayout{WindowID: 99, Children: []tcomponent.TileLayout{}}
				require.NoError(t, saveWorkspaceLayout(context.Background(), store, workspaceURI, complexLayout))
				require.NoError(t, saveWorkspaceLayout(context.Background(), store, otherWorkspaceURI, otherLayout))

				require.NoError(t, clearWorkspaceLayout(context.Background(), store, workspaceURI))
				require.NoError(t, clearWorkspaceLayout(context.Background(), store, workspaceURI))

				loaded, ok, err := loadWorkspaceLayout(context.Background(), store, workspaceURI)
				require.NoError(t, err)
				require.False(t, ok)
				require.Equal(t, tcomponent.TileLayout{}, loaded)

				loaded, ok, err = loadWorkspaceLayout(context.Background(), store, otherWorkspaceURI)
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, otherLayout, loaded)
			},
		},
		{
			name: "shares ide partition with terminal sessions",
			run: func(t *testing.T) {
				store := storagestub.NewInMemoryService()
				require.NoError(t, store.Set(context.Background(), terminalSessionDocumentID("manual"),
					terminalSessionDocument{Kind: terminalSessionDocumentKind, Name: "manual", Snapshot: vte.Snapshot{Version: 1}}))
				require.NoError(t, saveWorkspaceLayout(context.Background(), store, workspaceURI, complexLayout))

				loaded, ok, err := loadWorkspaceLayout(context.Background(), store, workspaceURI)
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, complexLayout, loaded)

				var terminalDoc terminalSessionDocument
				require.NoError(t, store.Get(context.Background(), terminalSessionDocumentID("manual"), &terminalDoc))
				require.Equal(t, "manual", terminalDoc.Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}
