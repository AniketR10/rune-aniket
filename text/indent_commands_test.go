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
	"unstable.build/go-tui/cell"
)

func TestSubscribeIndentCommands(t *testing.T) {
	cwd, err := workspaceapi.ParseURI("memory:///")
	require.NoError(t, err)
	uri := workspaceapi.Join(cwd, "indent.go")

	registry := newIndentTestWorkspaceRegistry()
	fileRegistry := NewFileCommandRegistry(cwd, registry)
	h := &indentTestHandler{uri: uri}
	c := setupCursorContent(t, 80, 10, "a", false)
	mock := &mockIndentService{returnIndentationAt: 1}
	mock.View = c.buffer().WithView(mock)

	wrapped, err := SubscribeIndentCommands(uri, fileRegistry, c, IndentConfig{}, h)
	require.NoError(t, err)
	commands := registry.sub[cwd.String()]
	require.Contains(t, commands, CommandReindent)

	err = commands[CommandReindent].HandleCommand(context.Background(), textapi.Command{
		URI:  uri,
		Name: CommandReindent,
	})
	require.NoError(t, err)
	assert.Equal(t, "\ta", c.buffer().String())

	require.NoError(t, wrapped.Close())
	assert.NotContains(t, registry.sub[cwd.String()], CommandReindent)
}

func TestIndentCommandReindentsSelection(t *testing.T) {
	cwd, err := workspaceapi.ParseURI("memory:///")
	require.NoError(t, err)
	uri := workspaceapi.Join(cwd, "indent.go")

	registry := newIndentTestWorkspaceRegistry()
	fileRegistry := NewFileCommandRegistry(cwd, registry)
	h := &indentTestHandler{uri: uri}
	c := setupCursorContent(t, 80, 10, "a\n\t\tb", false)
	mock := &mockIndentService{returnIndentationAt: 1}
	mock.View = c.buffer().WithView(mock)

	wrapped, err := SubscribeIndentCommands(uri, fileRegistry, c, IndentConfig{}, h)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, wrapped.Close()) })

	require.True(t, c.SelectRange(term.Coordinates{}, term.Coordinates{Y: 1}))
	err = registry.sub[cwd.String()][CommandReindent].HandleCommand(context.Background(), textapi.Command{
		URI:  uri,
		Name: CommandReindent,
	})
	require.NoError(t, err)
	assert.Equal(t, "\ta\n\tb", c.buffer().String())
	mode, ok := c.SelectionMode()
	assert.False(t, ok)
	assert.Equal(t, NoSelection, mode)
}

func TestIndentCommandWithoutIndentServiceReturnsError(t *testing.T) {
	cwd, err := workspaceapi.ParseURI("memory:///")
	require.NoError(t, err)
	uri := workspaceapi.Join(cwd, "indent.go")

	registry := newIndentTestWorkspaceRegistry()
	fileRegistry := NewFileCommandRegistry(cwd, registry)
	h := &indentTestHandler{uri: uri}
	c := setupCursorContent(t, 80, 10, "a", false)

	wrapped, err := SubscribeIndentCommands(uri, fileRegistry, c, IndentConfig{}, h)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, wrapped.Close()) })

	err = registry.sub[cwd.String()][CommandReindent].HandleCommand(context.Background(), textapi.Command{
		URI:  uri,
		Name: CommandReindent,
	})
	require.EqualError(t, err, "auto-indentation not available at the current position")
}

type indentTestWorkspaceRegistry struct {
	sub map[string]map[string]CommandHandler
}

func newIndentTestWorkspaceRegistry() *indentTestWorkspaceRegistry {
	return &indentTestWorkspaceRegistry{sub: make(map[string]map[string]CommandHandler)}
}

func (r *indentTestWorkspaceRegistry) SubscribeCommandForWorkspace(
	workspace workspaceapi.URI, cmd textapi.CommandManual, handler CommandHandler,
) error {
	if r.sub[workspace.String()] == nil {
		r.sub[workspace.String()] = make(map[string]CommandHandler)
	}
	r.sub[workspace.String()][cmd.Name] = handler
	return nil
}

func (r *indentTestWorkspaceRegistry) UnsubscribeCommandForWorkspace(
	workspace workspaceapi.URI, name string,
) error {
	if r.sub[workspace.String()] != nil {
		delete(r.sub[workspace.String()], name)
	}
	return nil
}

type indentTestHandler struct {
	handler.TestHandler
	uri workspaceapi.URI
}

func (h *indentTestHandler) Close() error { return nil }

func (h *indentTestHandler) Resource() workspaceapi.URI { return h.uri }

func (h *indentTestHandler) SetWrap(bool) {}

func (h *indentTestHandler) ShowCommandBar(bool) {}

func (h *indentTestHandler) SetCursorAtScroll(term.Coordinates) bool { return false }

func (h *indentTestHandler) CursorAtScroll() term.Coordinates { return term.Coordinates{} }

func (h *indentTestHandler) SetLocationList(textapi.LocationPriority, string, LocationList) {}

func (h *indentTestHandler) LocationLists() []LocationSet { return nil }

func (h *indentTestHandler) MoveToNextLocation(string) bool { return false }

func (h *indentTestHandler) MoveToPrevLocation(string) bool { return false }

func (h *indentTestHandler) CellView() cell.View { return nil }

func (h *indentTestHandler) CellEditor() cell.Editor { return nil }

func (h *indentTestHandler) SetDefaultAttributes(term.Attributes) {}

func (h *indentTestHandler) SeekUp() bool { return false }

func (h *indentTestHandler) SeekDown() bool { return false }

func (h *indentTestHandler) SeekOffset() int { return 0 }

func (h *indentTestHandler) MaxSeekOffset() int { return 0 }

func (h *indentTestHandler) Dimensions() (int, int) { return 0, 0 }

var _ Handler = (*indentTestHandler)(nil)
var _ browserapi.Handler = (*indentTestHandler)(nil)
