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

func TestSubscribeCommentCommands(t *testing.T) {
	cwd, err := workspaceapi.ParseURI("memory:///")
	require.NoError(t, err)
	uri := workspaceapi.Join(cwd, "comment.go")

	registry := newIndentTestWorkspaceRegistry()
	fileRegistry := NewFileCommandRegistry(cwd, registry)
	h := &indentTestHandler{uri: uri}
	c := setupCursorContent(t, 80, 10, "a\nb", false)
	attachCommentTestView(c, CommentSpec{
		Line:  []string{"//"},
		Block: []CommentBlock{{Start: "/*", End: "*/"}},
	})

	wrapped, err := SubscribeCommentCommands(uri, fileRegistry, c, h)
	require.NoError(t, err)
	commands := registry.sub[cwd.String()]
	require.Contains(t, commands, CommandToggleLineComment)
	require.Contains(t, commands, CommandToggleBlockComment)

	err = commands[CommandToggleLineComment].HandleCommand(context.Background(), textapi.Command{
		URI:  uri,
		Name: CommandToggleLineComment,
	})
	require.NoError(t, err)
	assert.Equal(t, "// a\nb", c.buffer().String())

	require.True(t, c.SelectRange(term.Coordinates{Y: 1, X: 0}, term.Coordinates{Y: 1, X: 1}))
	err = commands[CommandToggleBlockComment].HandleCommand(context.Background(), textapi.Command{
		URI:  uri,
		Name: CommandToggleBlockComment,
	})
	require.NoError(t, err)
	assert.Equal(t, "// a\n/*b*/", c.buffer().String())

	require.NoError(t, wrapped.Close())
	assert.NotContains(t, registry.sub[cwd.String()], CommandToggleLineComment)
	assert.NotContains(t, registry.sub[cwd.String()], CommandToggleBlockComment)
}

func TestCommentCommandLineWithoutSpecReturnsError(t *testing.T) {
	cwd, err := workspaceapi.ParseURI("memory:///")
	require.NoError(t, err)
	uri := workspaceapi.Join(cwd, "comment.go")

	registry := newIndentTestWorkspaceRegistry()
	fileRegistry := NewFileCommandRegistry(cwd, registry)
	h := &indentTestHandler{uri: uri}
	c := setupCursorContent(t, 80, 10, "a", false)

	wrapped, err := SubscribeCommentCommands(uri, fileRegistry, c, h)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, wrapped.Close()) })

	err = registry.sub[cwd.String()][CommandToggleLineComment].HandleCommand(
		context.Background(), textapi.Command{URI: uri, Name: CommandToggleLineComment})
	require.EqualError(t, err, "line comment not configured for this file")
}

func TestCommentCommandBlockWithoutSpecReturnsError(t *testing.T) {
	cwd, err := workspaceapi.ParseURI("memory:///")
	require.NoError(t, err)
	uri := workspaceapi.Join(cwd, "comment.go")

	registry := newIndentTestWorkspaceRegistry()
	fileRegistry := NewFileCommandRegistry(cwd, registry)
	h := &indentTestHandler{uri: uri}
	c := setupCursorContent(t, 80, 10, "a", false)

	wrapped, err := SubscribeCommentCommands(uri, fileRegistry, c, h)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, wrapped.Close()) })

	err = registry.sub[cwd.String()][CommandToggleBlockComment].HandleCommand(
		context.Background(), textapi.Command{URI: uri, Name: CommandToggleBlockComment})
	require.EqualError(t, err, "block comment not available at the current position")
}
