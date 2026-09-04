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

// SubscribeFoldCommands returns a slice of fold list related commands
// that can be registered for the returned CommandHandler.
func SubscribeFoldCommands(
	file workspaceapi.URI, registry FileCommandRegistry,
	cursor *Cursor, Handler Handler,
) (Handler, error) {
	ret := foldCommandHandler{
		file:     file,
		cursor:   cursor,
		registry: registry,
		Handler:  Handler,
	}
	var retErr error
	for _, cmd := range foldCommands {
		if err := registry.SubscribeCommandForFile(file, cmd, ret); err != nil {
			retErr = multierror.Append(retErr, err)
		}
	}
	if retErr != nil {
		return nil, retErr
	}
	return ret, nil
}

const (
	commandExpandFold       = "foldexpand"
	commandCollapseFold     = "foldcollapse"
	commandToggleFold       = "foldtoggle"
	commandExpandAllFolds   = "foldexpandall"
	commandCollapseAllFolds = "foldcollapseall"
	commandToggleAllFolds   = "foldtoggleall"
)

var foldCommands = []textapi.CommandManual{
	{
		Name:     commandExpandFold,
		Summary:  "Expand the collapsed fold surrounding the current cursor position.",
		Synopsis: "",
	},
	{
		Name:     commandCollapseFold,
		Summary:  "Collapses the expanded fold surrounding the current cursor position.",
		Synopsis: "",
	},
	{
		Name:     commandToggleFold,
		Summary:  "Collapses or expands the fold surrounding the current cursor position.",
		Synopsis: "",
	},
	{
		Name:     commandExpandAllFolds,
		Summary:  "Expand all folds in the file.",
		Synopsis: "",
	},
	{
		Name:     commandCollapseAllFolds,
		Summary:  "Collapses all folds in the file.",
		Synopsis: "",
	},
	{
		Name:     commandToggleAllFolds,
		Summary:  "Collapses or expands all folds in the file.",
		Synopsis: "",
	},
}

type foldCommandHandler struct {
	Handler
	cursor   *Cursor
	file     workspaceapi.URI
	registry FileCommandRegistry
}

func (u foldCommandHandler) Close() (ret error) {
	ret = u.Handler.Close()
	for _, cmd := range foldCommands {
		err := u.registry.UnsubscribeCommandForFile(u.file, cmd.Name)
		if err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return
}

func (u foldCommandHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) (err error) {
	var ok, all bool
	switch cmd.Name {
	case commandCollapseFold:
		ok = u.cursor.CollapseFold(ctx)
	case commandExpandFold:
		ok = u.cursor.ExpandFold(ctx)
	case commandToggleFold:
		ok = u.cursor.ToggleFold(ctx)
	case commandExpandAllFolds:
		ok = u.cursor.ExpandAllFolds(ctx)
		all = true
	case commandCollapseAllFolds:
		ok = u.cursor.CollapseAllFolds(ctx)
		all = true
	case commandToggleAllFolds:
		ok = u.cursor.ToggleAllFolds(ctx)
		all = true
	default:
		err = errors.New("extraneous command")
	}
	if !ok {
		if all {
			return errors.New("no folds in this file")
		} else {
			return errors.New("no folds at the current position")
		}
	}
	return
}

func (u foldCommandHandler) Complete(ctx context.Context, cmd textapi.Command) (
	ret iterator.Iterator[string], _ string, err error,
) {
	ret = iterator.Empty[string]()
	return
}
