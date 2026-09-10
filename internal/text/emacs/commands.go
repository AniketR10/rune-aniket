// Copyright (C) 2017-2026 The Rune Authors
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

package emacs

import (
	"context"
	"errors"

	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/internal/text"
)

// CommandUndo reverses the most recent buffer change.
const CommandUndo = "undo"

const undoPrefixArg = "prefix"

var undoCommand = textapi.CommandManual{
	Name:    CommandUndo,
	Summary: "Undo the most recent buffer change.",
}

func subscribeUndoCommand(
	file workspaceapi.URI, registry text.FileCommandRegistry,
	h *emacsHandler, wrapped text.Handler,
) (text.Handler, error) {
	ret := undoCommandHandler{
		Handler:  wrapped,
		h:        h,
		file:     file,
		registry: registry,
	}
	if err := registry.SubscribeCommandForFile(file, undoCommand, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

type undoCommandHandler struct {
	text.Handler
	h        *emacsHandler
	file     workspaceapi.URI
	registry text.FileCommandRegistry
}

func (h undoCommandHandler) Close() (ret error) {
	ret = h.Handler.Close()
	if err := h.registry.UnsubscribeCommandForFile(h.file, CommandUndo); err != nil {
		return errors.Join(ret, err)
	}
	return ret
}

func (h undoCommandHandler) HandleCommand(
	_ context.Context, cmd textapi.Command,
) error {
	if len(cmd.Args) == 1 && cmd.Args[0] == undoPrefixArg {
		h.h.undoFromPrefix()
		return nil
	}
	h.h.undoPrefix = nil
	h.h.undo()
	return nil
}

func (h undoCommandHandler) Complete(context.Context, textapi.Command) (
	iterator.Iterator[string], string, error,
) {
	return iterator.Empty[string](), "", nil
}
