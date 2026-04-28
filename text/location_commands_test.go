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

package text

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/cell"
)

func TestLocationHighlightCommandHighlightsAllLocations(t *testing.T) {
	cwd, err := workspaceapi.ParseURI("memory:///")
	require.NoError(t, err)
	uri := workspaceapi.Join(cwd, "bookmarks.go")

	registry := newIndentTestWorkspaceRegistry()
	fileRegistry := NewFileCommandRegistry(cwd, registry)
	bookmarks := []textapi.Location{
		{
			From: term.Coordinates{Y: 0},
			To:   term.Coordinates{Y: 0, X: 1},
		},
		{
			From: term.Coordinates{Y: 2},
			To:   term.Coordinates{Y: 2, X: 1},
		},
	}
	h := &locationCommandTestHandler{
		uri: uri,
		lists: []LocationSet{{
			ID:        "bookmark",
			Priority:  textapi.LocationPriorityInfo,
			Locations: bookmarks,
		}},
	}

	wrapped, err := SubscribeLocationCommands(uri, fileRegistry, h)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, wrapped.Close()) })

	err = registry.sub[cwd.String()][CommandHighlightLocations].HandleCommand(context.Background(), textapi.Command{
		URI:  uri,
		Name: CommandHighlightLocations,
		Args: []string{"bookmark"},
	})
	require.NoError(t, err)

	assert.Equal(t, textapi.LocationPriorityCritical, h.lastPriority)
	assert.Equal(t, selectionLocationListID, h.lastID)
	assert.False(t, h.lastLocationListNil)
	assert.Equal(t, []textapi.Location{
		{
			From: term.Coordinates{Y: 0},
			To:   term.Coordinates{Y: 0, X: 1},
			Attr: term.Attributes{Attrs: tcell.AttrReverse},
		},
		{
			From: term.Coordinates{Y: 2},
			To:   term.Coordinates{Y: 2, X: 1},
			Attr: term.Attributes{Attrs: tcell.AttrReverse},
		},
	}, h.lastLocations)

	err = registry.sub[cwd.String()][CommandHighlightLocations].HandleCommand(context.Background(), textapi.Command{
		URI:  uri,
		Name: CommandHighlightLocations,
		Args: []string{"bookmark"},
	})
	require.NoError(t, err)

	assert.Equal(t, textapi.LocationPriorityCritical, h.lastPriority)
	assert.Equal(t, selectionLocationListID, h.lastID)
	assert.False(t, h.lastLocationListNil)
	assert.Empty(t, h.lastLocations)
}

type locationCommandTestHandler struct {
	handler.TestHandler
	uri                 workspaceapi.URI
	lists               []LocationSet
	lastPriority        textapi.LocationPriority
	lastID              string
	lastLocations       []textapi.Location
	lastLocationListNil bool
}

func (h *locationCommandTestHandler) Close() error { return nil }

func (h *locationCommandTestHandler) Resource() workspaceapi.URI { return h.uri }

func (h *locationCommandTestHandler) SetWrap(bool) {}

func (h *locationCommandTestHandler) ShowCommandBar(bool) {}

func (h *locationCommandTestHandler) SetCursorAtScroll(term.Coordinates) bool { return false }

func (h *locationCommandTestHandler) CursorAtScroll() term.Coordinates { return term.Coordinates{} }

func (h *locationCommandTestHandler) SetLocationList(
	priority textapi.LocationPriority, id string, locations LocationList,
) {
	h.lastPriority = priority
	h.lastID = id
	h.lastLocationListNil = locations == nil
	if locations == nil {
		h.lastLocations = nil
		for i, list := range h.lists {
			if list.ID == id {
				h.lists = append(h.lists[:i], h.lists[i+1:]...)
				return
			}
		}
		return
	}
	h.lastLocations = []textapi.Location{}
	scrollStartList(locations)
	for loc, ok := locations.Current(); ok; loc, ok = locations.Next() {
		h.lastLocations = append(h.lastLocations, loc)
	}
	for i, list := range h.lists {
		if list.ID == id {
			h.lists[i].Priority = priority
			h.lists[i].Locations = append([]textapi.Location(nil), h.lastLocations...)
			return
		}
	}
	h.lists = append(h.lists, LocationSet{
		ID:        id,
		Priority:  priority,
		Locations: append([]textapi.Location(nil), h.lastLocations...),
	})
}

func (h *locationCommandTestHandler) LocationLists() []LocationSet { return h.lists }

func (h *locationCommandTestHandler) MoveToNextLocation(string) bool { return false }

func (h *locationCommandTestHandler) MoveToPrevLocation(string) bool { return false }

func (h *locationCommandTestHandler) CellView() cell.View { return nil }

func (h *locationCommandTestHandler) CellEditor() cell.Editor { return nil }

func (h *locationCommandTestHandler) SetDefaultAttributes(term.Attributes) {}

func (h *locationCommandTestHandler) SeekUp() bool { return false }

func (h *locationCommandTestHandler) SeekDown() bool { return false }

func (h *locationCommandTestHandler) SeekOffset() int { return 0 }

func (h *locationCommandTestHandler) MaxSeekOffset() int { return 0 }

func (h *locationCommandTestHandler) Dimensions() (int, int) { return 0, 0 }

var _ Handler = (*locationCommandTestHandler)(nil)
var _ browserapi.Handler = (*locationCommandTestHandler)(nil)
