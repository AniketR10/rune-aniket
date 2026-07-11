// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

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
