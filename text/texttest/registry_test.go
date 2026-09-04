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

package texttest

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/text"
)

func TestRegistry(t *testing.T) {
	var abc, xyz int
	nopHandler := text.FuncCommandHandler(func(_ context.Context, cmd textapi.Command) error {
		switch cmd.Name {
		case "abc":
			abc++
		case "xyz":
			xyz++
		default:
			return errors.New("invalid command")
		}
		return nil
	}, nil)
	cwd1, err := workspaceapi.ParseURI("memory:///1")
	require.NoError(t, err)
	wr := newWorkspaceRegistry()
	fr := text.NewFileCommandRegistry(cwd1, &wr)

	xyzman := textapi.CommandManual{Name: "xyz"}
	abcman := textapi.CommandManual{Name: "abc"}

	file1 := workspaceapi.Join(cwd1, "file1")
	file2 := workspaceapi.Join(cwd1, "file2")
	file3 := workspaceapi.Join(cwd1, "file3")

	require.NoError(t, fr.SubscribeCommandForFile(file1, xyzman, nopHandler))
	require.NoError(t, fr.SubscribeCommandForFile(file2, xyzman, nopHandler))
	require.NoError(t, fr.SubscribeCommandForFile(file1, abcman, nopHandler))
	require.NoError(t, fr.SubscribeCommandForFile(file2, abcman, nopHandler))

	cmds := wr.sub[cwd1.String()]
	require.Len(t, cmds, 2)
	handler1, ok := cmds["xyz"]
	require.True(t, ok)
	handler2, ok := cmds["abc"]
	require.True(t, ok)

	ctx := context.Background()
	require.NoError(t, handler1.HandleCommand(ctx, textapi.Command{URI: file1, Name: "xyz"}))
	require.NoError(t, handler2.HandleCommand(ctx, textapi.Command{URI: file1, Name: "abc"}))
	require.NoError(t, handler1.HandleCommand(ctx, textapi.Command{URI: file2, Name: "xyz"}))
	require.NoError(t, handler2.HandleCommand(ctx, textapi.Command{URI: file2, Name: "abc"}))
	require.Error(t, handler1.HandleCommand(ctx, textapi.Command{URI: file3, Name: "xyz"}))
	require.Error(t, handler2.HandleCommand(ctx, textapi.Command{URI: file3, Name: "abc"}))
	require.Error(t, handler2.HandleCommand(ctx, textapi.Command{URI: file1, Name: "else"}))

	require.NoError(t, fr.UnsubscribeCommandForFile(file2, xyzman.Name))
	require.NoError(t, fr.UnsubscribeCommandForFile(file1, abcman.Name))

	assert.Equal(t, 2, xyz)
	assert.Equal(t, 2, abc)

	require.NoError(t, handler1.HandleCommand(ctx, textapi.Command{URI: file1, Name: "xyz"}))
	require.Error(t, handler2.HandleCommand(ctx, textapi.Command{URI: file1, Name: "abc"}))
	require.Error(t, handler1.HandleCommand(ctx, textapi.Command{URI: file2, Name: "xyz"}))
	require.NoError(t, handler2.HandleCommand(ctx, textapi.Command{URI: file2, Name: "abc"}))
	require.Error(t, handler1.HandleCommand(ctx, textapi.Command{URI: file3, Name: "xyz"}))
	require.Error(t, handler2.HandleCommand(ctx, textapi.Command{URI: file3, Name: "abc"}))
	require.Error(t, handler2.HandleCommand(ctx, textapi.Command{URI: file1, Name: "else"}))

	assert.Equal(t, 3, xyz)
	assert.Equal(t, 3, abc)

	require.NoError(t, fr.UnsubscribeCommandForFile(file1, xyzman.Name))
	require.NoError(t, fr.UnsubscribeCommandForFile(file2, abcman.Name))

	require.Error(t, handler1.HandleCommand(ctx, textapi.Command{URI: file1, Name: "xyz"}))
	require.Error(t, handler2.HandleCommand(ctx, textapi.Command{URI: file1, Name: "abc"}))
	require.Error(t, handler1.HandleCommand(ctx, textapi.Command{URI: file2, Name: "xyz"}))
	require.Error(t, handler2.HandleCommand(ctx, textapi.Command{URI: file2, Name: "abc"}))
	require.Error(t, handler1.HandleCommand(ctx, textapi.Command{URI: file3, Name: "xyz"}))
	require.Error(t, handler2.HandleCommand(ctx, textapi.Command{URI: file3, Name: "abc"}))
	require.Error(t, handler2.HandleCommand(ctx, textapi.Command{URI: file1, Name: "else"}))

	assert.Equal(t, 3, xyz)
	assert.Equal(t, 3, abc)
}

type workspaceRegistry struct {
	sub map[string]map[string]text.CommandHandler
}

func newWorkspaceRegistry() workspaceRegistry {
	return workspaceRegistry{sub: make(map[string]map[string]text.CommandHandler)}
}

func (t workspaceRegistry) SubscribeCommandForWorkspace(
	workspace workspaceapi.URI, cmd textapi.CommandManual, handler text.CommandHandler) error {
	subs, ok := t.sub[workspace.String()]
	if ok {
		if _, ok := subs[cmd.Name]; ok {
			return errors.New("already registered")
		}
		subs[cmd.Name] = handler
	} else {
		t.sub[workspace.String()] = map[string]text.CommandHandler{cmd.Name: handler}
	}
	return nil
}

func (t workspaceRegistry) UnsubscribeCommandForWorkspace(workspace workspaceapi.URI, name string) error {
	delete(t.sub[workspace.String()], name)
	return nil
}
