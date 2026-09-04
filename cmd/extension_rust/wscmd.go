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

package main

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// wsRequestCmd sends a workspace-scoped rust-analyzer request that has no
// meaningful result (reloadWorkspace, rebuildProcMacros) and reports a
// done notification.
type wsRequestCmd struct {
	lsp    semanticapi.LSP
	notify browserapi.Notifications
	method string
	done   string
}

var _ textapi.CommandHandler = (*wsRequestCmd)(nil)

func (c *wsRequestCmd) HandleCommand(ctx context.Context, _ textapi.Command) error {
	if _, err := execRequest[any](ctx, c.lsp, c.method, nil); err != nil {
		return err
	}
	_, _ = c.notify.Notify(browserapi.LevelInfo, "%s", c.done)
	return nil
}

func (c *wsRequestCmd) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

// flycheckKind selects which flycheck notification a flycheckCmd sends.
type flycheckKind int

const (
	flycheckRun flycheckKind = iota
	flycheckClear
	flycheckCancel
)

// flycheckCmd sends a flycheck notification. `run` scopes to the current
// document when one is open (null runs every configured check); `clear`
// and `cancel` take an empty payload.
type flycheckCmd struct {
	lsp    semanticapi.LSP
	notify browserapi.Notifications
	kind   flycheckKind
	done   string
}

var _ textapi.CommandHandler = (*flycheckCmd)(nil)

func (c *flycheckCmd) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	var method string
	var params any = struct{}{}
	switch c.kind {
	case flycheckRun:
		method = "rust-analyzer/runFlycheck"
		var doc *semanticapi.TextDocumentIdentifier
		if cmd.Resource != nil {
			d := docParams(cmd)
			doc = &d
		}
		params = struct {
			TextDocument *semanticapi.TextDocumentIdentifier `json:"textDocument"`
		}{TextDocument: doc}
	case flycheckClear:
		method = "rust-analyzer/clearFlycheck"
	case flycheckCancel:
		method = "rust-analyzer/cancelFlycheck"
	}
	if err := sendNotification(ctx, c.lsp, method, params); err != nil {
		return err
	}
	_, _ = c.notify.Notify(browserapi.LevelInfo, "%s", c.done)
	return nil
}

func (c *flycheckCmd) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}
