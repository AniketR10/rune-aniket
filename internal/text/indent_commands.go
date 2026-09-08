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

// CommandReindent is the command name for reindenting the current line or selection.
const CommandReindent = "reindent"

var indentCommands = []textapi.CommandManual{{
	Name:     CommandReindent,
	Summary:  "Reindents the current line or the active selection using the syntax tree.",
	Synopsis: "",
}}

// SubscribeIndentCommands returns indent-related commands that can be registered
// for the returned Handler.
func SubscribeIndentCommands(
	file workspaceapi.URI, registry FileCommandRegistry,
	cursor *Cursor, indents IndentConfig, handler Handler,
) (Handler, error) {
	ret := indentCommandHandler{
		file:     file,
		cursor:   cursor,
		indents:  indents,
		registry: registry,
		Handler:  handler,
	}
	var retErr error
	for _, cmd := range indentCommands {
		if err := registry.SubscribeCommandForFile(file, cmd, ret); err != nil {
			retErr = multierror.Append(retErr, err)
		}
	}
	if retErr != nil {
		return nil, retErr
	}
	return ret, nil
}

type indentCommandHandler struct {
	Handler
	cursor   *Cursor
	file     workspaceapi.URI
	indents  IndentConfig
	registry FileCommandRegistry
}

func (u indentCommandHandler) Close() (ret error) {
	ret = u.Handler.Close()
	for _, cmd := range indentCommands {
		err := u.registry.UnsubscribeCommandForFile(u.file, cmd.Name)
		if err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return
}

func (u indentCommandHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) (err error) {
	switch cmd.Name {
	case CommandReindent:
		indentRune, tabspaces, ok := IndentConfigForURI(
			u.file, u.cursor.buffer(), u.indents,
			max(1, u.cursor.scroll.Tabspaces()),
		)
		if !ok {
			indentRune = IndentRuneTab
		}
		if _, ok := u.cursor.SelectionMode(); ok {
			u.cursor.ReindentSelection(indentRune, tabspaces)
			return nil
		}
		if !u.cursor.Reindent(indentRune, tabspaces) {
			return errors.New("auto-indentation not available at the current position")
		}
		return nil
	default:
		return errors.New("extraneous command")
	}
}

func (u indentCommandHandler) Complete(ctx context.Context, cmd textapi.Command) (
	ret iterator.Iterator[string], _ string, err error,
) {
	ret = iterator.Empty[string]()
	return
}
