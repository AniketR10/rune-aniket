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
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// FormatHandler creates a textapi.CommandHandler for the "format" subcommand.
// It subscribes to selection and cursor events to track active selections.
func FormatHandler(
	lsp semanticapi.LSP, editor textapi.Editor,
) (textapi.CommandHandler, error) {
	evs := []textapi.EventType{textapi.EventTypeSelection, textapi.EventTypeCursor}
	sel := NewSelectionTracker()
	err := editor.SubscribeEvents(evs, sel)
	if err != nil {
		return nil, err
	}
	return &formatHandler{lsp: lsp, editor: editor, sel: sel}, nil
}

var _ textapi.CommandHandler = (*formatHandler)(nil)

type formatHandler struct {
	lsp    semanticapi.LSP
	editor textapi.Editor
	sel    *SelectionTracker
}

func (h *formatHandler) HandleCommand(
	ctx context.Context, cmd textapi.Command,
) error {
	if cmd.Resource == nil {
		return fmt.Errorf("no file open; open a file first")
	}
	_, hasSelection := cmd.Resource.Selection()
	if hasSelection {
		return h.formatRange(ctx, cmd)
	}
	return h.formatFull(ctx, cmd)
}

func (h *formatHandler) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

func (h *formatHandler) formatFull(
	ctx context.Context, cmd textapi.Command,
) error {
	params := semanticapi.DocumentFormattingParams{
		TextDocument: TextDocID(cmd.URI),
		Options: semanticapi.FormattingOptions{
			TabSize:      4,
			InsertSpaces: false,
		},
	}
	edits, err := h.lsp.Formatting(ctx, params)
	if err != nil {
		return err
	}
	if len(edits) == 0 {
		return nil
	}
	ce := h.editor.CellEditor(cmd.Resource)
	return ApplyEdits(ctx, ce, edits)
}

func (h *formatHandler) formatRange(
	ctx context.Context, cmd textapi.Command,
) error {
	selRange, ok := h.sel.Get(cmd.URI)
	if !ok {
		return h.formatFull(ctx, cmd)
	}
	params := semanticapi.DocumentRangeFormattingParams{
		TextDocument: TextDocID(cmd.URI),
		Range:        selRange,
		Options: semanticapi.FormattingOptions{
			TabSize:      4,
			InsertSpaces: false,
		},
	}
	edits, err := h.lsp.RangeFormatting(ctx, params)
	if err != nil {
		return err
	}
	if len(edits) == 0 {
		return nil
	}
	ce := h.editor.CellEditor(cmd.Resource)
	return ApplyEdits(ctx, ce, edits)
}
