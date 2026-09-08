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

package text

import (
	"context"
	"errors"
	"fmt"

	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// NewFileCommandRegistry returns a command registry that satisfies FileCommandRegistry,
// by using the given workspace-wide registry to register commands.
func NewFileCommandRegistry(
	workspace workspaceapi.URI, registry WorkspaceCommandRegistry,
) FileCommandRegistry {
	ret := new(fileCmdRegistry)
	ret.cmds = make(map[string]map[string]commandAll)
	ret.register = func(cmd textapi.CommandManual, c CommandHandler) error {
		return registry.SubscribeCommandForWorkspace(workspace, cmd, c)
	}
	ret.unregister = func(name string) error {
		return registry.UnsubscribeCommandForWorkspace(workspace, name)
	}
	return ret
}

func newFileCommandRegistryFromComponent(comp *Component) FileCommandRegistry {
	ret := new(fileCmdRegistry)
	ret.cmds = make(map[string]map[string]commandAll)
	ret.register = func(cmd textapi.CommandManual, c CommandHandler) error {
		return comp.SubscribeCommand(cmd, c)
	}
	ret.unregister = func(name string) error {
		return comp.UnsubscribeCommand(name)
	}
	return ret
}

type fileCmdRegistry struct {
	register   func(textapi.CommandManual, CommandHandler) error
	unregister func(string) error
	// keep track in adition to registry, so we can
	// unsubscribe on Close
	cmds map[string]map[string]commandAll
}

func (c *fileCmdRegistry) SubscribeCommandForFile(
	file workspaceapi.URI, cmd textapi.CommandManual, cm CommandHandler,
) error {
	cc := commandAll{
		man:     apiManualToManual(cmd),
		handler: cm,
	}
	files, ok := c.cmds[cmd.Name]
	if !ok {
		c.cmds[cmd.Name] = map[string]commandAll{
			file.String(): cc,
		}
		return c.register(cmd, c)
	}
	if _, ok := files[file.String()]; ok {
		return errors.New("command already registered")
	}
	files[file.String()] = cc
	return nil
}

func (c *fileCmdRegistry) UnsubscribeCommandForFile(uri workspaceapi.URI, cmd string) error {
	files, ok := c.cmds[cmd]
	if !ok {
		return errors.New("command not registered")
	}
	_, ok = files[uri.String()]
	if !ok {
		return errors.New("command not registered")
	}
	if len(files) == 1 {
		delete(c.cmds, cmd)
		return c.unregister(cmd)
	}
	delete(files, uri.String())
	return nil
}

// HandleCommand satisfies CommandHandler by dispatching command to file-level
// command handlers registered via SubscribeCommandForFile.
func (c *fileCmdRegistry) HandleCommand(ctx context.Context, cmd textapi.Command) (ret error) {
	files, ok := c.cmds[cmd.Name]
	if !ok {
		ret = errors.New("dispatched command but publisher was already unsubscribed")
		return
	}
	handler, ok := files[cmd.URI.String()]
	if !ok {
		// command is being dispatched on a file that doesn't have a handler for it
		// could be by user error so show a reassuring message.
		ret = errors.New("this file doesn't support this command")
		return
	}
	return handler.handler.HandleCommand(ctx, cmd)
}

// Complete satisfies CommandHandler by dispatching complete request to
// file-level command handlers registered via SubscribeCommandForFile.
func (c *fileCmdRegistry) Complete(ctx context.Context, cmd textapi.Command) (
	iterator.Iterator[string], string, error,
) {
	files, ok := c.cmds[cmd.Name]
	if !ok {
		return iterator.Empty[string](), "", nil
	}
	handler, ok := files[cmd.URI.String()]
	if !ok {
		return nil, "", fmt.Errorf("this file doesn't support this command")
	}
	return handler.handler.Complete(ctx, cmd)
}
