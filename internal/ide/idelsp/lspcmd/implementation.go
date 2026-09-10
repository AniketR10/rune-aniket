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

// ImplementationConfig configures the "implementation" subcommand.
type ImplementationConfig struct {
	ListConfig locationpicker.Config
}

// DefaultImplementationConfig returns an ImplementationConfig with sensible defaults.
func DefaultImplementationConfig() ImplementationConfig {
	return ImplementationConfig{
		ListConfig: locationpicker.DefaultConfig(),
	}
}

// ImplementationHandler creates a textapi.CommandHandler that finds
// implementations of the symbol at the cursor position and displays them in a
// floating window with a file preview.
func ImplementationHandler(
	lsp semanticapi.LSP, editor textapi.Editor,
	wm browserapi.WindowManager, opener browserapi.ResourceOpener,
	notify browserapi.Notifications, fs workspaceapi.FileSystem,
	rootURI workspaceapi.URI, scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser, cfg ImplementationConfig,
	log *slog.Logger,
) textapi.CommandHandler {
	return &implementationHandler{
		lsp: lsp, editor: editor, wm: wm, opener: opener,
		notify: notify, fs: fs, rootURI: rootURI,
		scheduleNextTick: scheduleNextTick, parser: parser, cfg: cfg,
		log: log,
	}
}

type implementationHandler struct {
	lsp              semanticapi.LSP
	editor           textapi.Editor
	wm               browserapi.WindowManager
	opener           browserapi.ResourceOpener
	notify           browserapi.Notifications
	fs               workspaceapi.FileSystem
	rootURI          workspaceapi.URI
	scheduleNextTick func(func()) bool
	parser           syntaxapi.Parser
	cfg              ImplementationConfig
	log              *slog.Logger
}

func (h *implementationHandler) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	proceed, err := resolveCommandSymbol(ctx, &cmd, h.rootURI, h.wm, h.fs, h.notify, h.scheduleNextTick, h.parser,
		executeResolved("implementation", h.rootURI, h.notify, h.scheduleNextTick, h.execute))
	if !proceed || err != nil {
		return err
	}
	executeAtCursor("implementation", h.notify, h.scheduleNextTick, cmd, h.execute)
	return nil
}

// execute performs the blocking LSP round trip. It is called off the event
// loop and schedules UI work onto it.
func (h *implementationHandler) execute(
	ctx context.Context, uri workspaceapi.URI, pos semanticapi.Position,
) error {
	params := semanticapi.ImplementationParams{
		TextDocument: TextDocID(uri),
		Position:     pos,
	}
	result, err := h.lsp.Implementation(ctx, params)
	if err != nil {
		return err
	}
	entries := locationsFromResult(result)
	if len(entries) == 0 {
		return errNoLocations
	}
	entries = enrichEntries(entries, h.rootURI)
	presentLocations(
		"implementation", h.rootURI, entries, h.opener, h.wm, h.editor, h.notify,
		h.fs, h.scheduleNextTick, h.parser, h.cfg.ListConfig, h.log,
	)
	return nil
}

func (h *implementationHandler) Complete(
	ctx context.Context, _ string, args []string,
) (iterator.Iterator[string], error) {
	return completeReferencedSymbol(ctx, h.parser)
}
