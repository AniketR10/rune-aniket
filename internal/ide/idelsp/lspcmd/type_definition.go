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

package lspcmd

import (
	"context"
	"log/slog"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/rune/internal/handler/locationpicker"
)

// TypeDefinitionConfig configures the "type-definition" subcommand.
type TypeDefinitionConfig struct {
	ListConfig locationpicker.Config
}

// DefaultTypeDefinitionConfig returns a TypeDefinitionConfig with sensible defaults.
func DefaultTypeDefinitionConfig() TypeDefinitionConfig {
	return TypeDefinitionConfig{}
}

// TypeDefinitionHandler creates a textapi.CommandHandler that finds the type
// definition of the symbol at the cursor position and displays the result in a
// floating window with a file preview.
func TypeDefinitionHandler(
	lsp semanticapi.LSP, editor textapi.Editor,
	wm browserapi.WindowManager, opener browserapi.ResourceOpener,
	notify browserapi.Notifications, fs workspaceapi.FileSystem,
	rootURI workspaceapi.URI, scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser, cfg TypeDefinitionConfig,
	log *slog.Logger,
) textapi.CommandHandler {
	return &typeDefinitionHandler{
		lsp: lsp, editor: editor, wm: wm, opener: opener,
		notify: notify, fs: fs, rootURI: rootURI,
		scheduleNextTick: scheduleNextTick, parser: parser, cfg: cfg,
		log: log,
	}
}

type typeDefinitionHandler struct {
	lsp              semanticapi.LSP
	editor           textapi.Editor
	wm               browserapi.WindowManager
	opener           browserapi.ResourceOpener
	notify           browserapi.Notifications
	fs               workspaceapi.FileSystem
	rootURI          workspaceapi.URI
	scheduleNextTick func(func()) bool
	parser           syntaxapi.Parser
	cfg              TypeDefinitionConfig
	log              *slog.Logger
}

func (h *typeDefinitionHandler) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	proceed, err := resolveCommandSymbol(ctx, &cmd, h.rootURI, h.wm, h.fs, h.notify, h.scheduleNextTick, h.parser,
		executeResolved("type-definition", h.rootURI, h.notify, h.scheduleNextTick, h.execute))
	if !proceed || err != nil {
		return err
	}
	executeAtCursor("type-definition", h.notify, h.scheduleNextTick, cmd, h.execute)
	return nil
}

// execute performs the blocking LSP round trip. It is called off the event
// loop and schedules UI work onto it.
func (h *typeDefinitionHandler) execute(
	ctx context.Context, uri workspaceapi.URI, pos semanticapi.Position,
) error {
	params := semanticapi.TypeDefinitionParams{
		TextDocument: TextDocID(uri),
		Position:     pos,
	}
	result, err := h.lsp.TypeDefinition(ctx, params)
	if err != nil {
		return err
	}
	entries := locationsFromResult(result)
	if len(entries) == 0 {
		return errNoLocations
	}
	entries = enrichEntries(entries, h.rootURI)
	presentLocations(
		"type-definition", h.rootURI, entries, h.opener, h.wm, h.editor, h.notify,
		h.fs, h.scheduleNextTick, h.parser, h.cfg.ListConfig, h.log,
	)
	return nil
}

func (h *typeDefinitionHandler) Complete(
	ctx context.Context, _ string, args []string,
) (iterator.Iterator[string], error) {
	return completeReferencedSymbol(ctx, h.parser)
}
