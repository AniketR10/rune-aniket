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

	"github.com/ernestrc/go-multierror"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// CommandToggleLineComment toggles the configured line-comment prefix on the
// current line or selected lines.
const CommandToggleLineComment = "commentlinetoggle"

// CommandToggleBlockComment toggles the configured block-comment delimiters
// around the current selection or block under the cursor.
const CommandToggleBlockComment = "commentblocktoggle"

var commentCommands = []textapi.CommandManual{
	{
		Name:     CommandToggleLineComment,
		Summary:  "Toggles the line comment prefix on the current line or selection.",
		Synopsis: "",
	},
	{
		Name:     CommandToggleBlockComment,
		Summary:  "Toggles block comment delimiters around the current selection.",
		Synopsis: "",
	},
}

// SubscribeCommentCommands returns comment-related commands that can be
// registered for the returned Handler.
func SubscribeCommentCommands(
	file workspaceapi.URI, registry FileCommandRegistry,
	cursor *Cursor, handler Handler,
) (Handler, error) {
	ret := commentCommandHandler{
		file:     file,
		cursor:   cursor,
		registry: registry,
		Handler:  handler,
	}
	var retErr error
	for _, cmd := range commentCommands {
		if err := registry.SubscribeCommandForFile(file, cmd, ret); err != nil {
			retErr = multierror.Append(retErr, err)
		}
	}
	if retErr != nil {
		return nil, retErr
	}
	return ret, nil
}

type commentCommandHandler struct {
	Handler
	cursor   *Cursor
	file     workspaceapi.URI
	registry FileCommandRegistry
}

func (u commentCommandHandler) Close() (ret error) {
	ret = u.Handler.Close()
	for _, cmd := range commentCommands {
		err := u.registry.UnsubscribeCommandForFile(u.file, cmd.Name)
		if err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return
}

func (u commentCommandHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) (err error) {
	switch cmd.Name {
	case CommandToggleLineComment:
		if !u.cursor.ToggleLineComment() {
			return errors.New("line comment not configured for this file")
		}
		return nil
	case CommandToggleBlockComment:
		if !u.cursor.ToggleBlockComment() {
			return errors.New("block comment not available at the current position")
		}
		return nil
	default:
		return errors.New("extraneous command")
	}
}

func (u commentCommandHandler) Complete(ctx context.Context, cmd textapi.Command) (
	ret iterator.Iterator[string], _ string, err error,
) {
	ret = iterator.Empty[string]()
	return
}
