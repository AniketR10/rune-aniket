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
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

const testMarkListID = "mark"

func TestExchangePointAndMarkSwapsCursorAndMark(t *testing.T) {
	cwd, err := workspaceapi.ParseURI("memory:///")
	require.NoError(t, err)
	uri := workspaceapi.Join(cwd, "mark.go")

	registry := newIndentTestWorkspaceRegistry()
	fileRegistry := NewFileCommandRegistry(cwd, registry)
	h := &indentTestHandler{uri: uri}
	c := setupCursorContent(t, 80, 10, "a\nb\nc\nd", false)

	wrapped, err := SubscribeMarkCommands(uri, fileRegistry, c, testMarkListID, h)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, wrapped.Close()) })

	commands := registry.sub[cwd.String()]
	require.Contains(t, commands, CommandExchangePointAndMark)

	// Mark at line 0, point at line 2.
	c.SetLocationList(textapi.LocationPriorityInfo, testMarkListID, LocationSlice(
		[]textapi.Location{{From: term.Coordinates{}, To: term.Coordinates{X: 1}}}))
	_, ok := c.MoveToScroll(term.Coordinates{Y: 2})
	require.True(t, ok)

	err = commands[CommandExchangePointAndMark].HandleCommand(context.Background(),
		textapi.Command{URI: uri, Name: CommandExchangePointAndMark})
	require.NoError(t, err)

	// Point moved to the old mark; the mark now records the old point.
	assert.Equal(t, term.Coordinates{}, c.CursorAtScroll())
	marks := markListLocations(c, testMarkListID)
	require.Len(t, marks, 1)
	assert.Equal(t, term.Coordinates{Y: 2}, marks[0].From)

	// A second exchange is the inverse and restores the original positions.
	err = commands[CommandExchangePointAndMark].HandleCommand(context.Background(),
		textapi.Command{URI: uri, Name: CommandExchangePointAndMark})
	require.NoError(t, err)
	assert.Equal(t, term.Coordinates{Y: 2}, c.CursorAtScroll())
	marks = markListLocations(c, testMarkListID)
	require.Len(t, marks, 1)
	assert.Equal(t, term.Coordinates{}, marks[0].From)
}

func TestExchangePointAndMarkWithoutMarkErrors(t *testing.T) {
	cwd, err := workspaceapi.ParseURI("memory:///")
	require.NoError(t, err)
	uri := workspaceapi.Join(cwd, "mark.go")

	registry := newIndentTestWorkspaceRegistry()
	fileRegistry := NewFileCommandRegistry(cwd, registry)
	h := &indentTestHandler{uri: uri}
	c := setupCursorContent(t, 80, 10, "a\nb", false)

	wrapped, err := SubscribeMarkCommands(uri, fileRegistry, c, testMarkListID, h)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, wrapped.Close()) })

	err = registry.sub[cwd.String()][CommandExchangePointAndMark].HandleCommand(
		context.Background(), textapi.Command{URI: uri, Name: CommandExchangePointAndMark})
	require.EqualError(t, err, "no mark set in this buffer")
}

func markListLocations(c *Cursor, id string) []textapi.Location {
	for _, list := range c.LocationLists() {
		if list.ID == id {
			return list.Locations
		}
	}
	return nil
}
