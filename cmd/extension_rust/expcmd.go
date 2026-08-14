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
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/ide/idelsp/lspcmd"
)

// hoverRangeCmd asks rust-analyzer for the type of the current selection
// using the experimental hoverRange extension, which accepts a Range in
// the hover request's position field. It falls back to the cursor
// position when nothing is selected.
type hoverRangeCmd struct {
	lsp       semanticapi.LSP
	wm        browserapi.WindowManager
	notify    browserapi.Notifications
	parser    syntaxapi.Parser
	interrupt term.Interrupter
	sel       *lspcmd.SelectionTracker
}

var _ textapi.CommandHandler = (*hoverRangeCmd)(nil)

func (c *hoverRangeCmd) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	if err := requireFile(cmd); err != nil {
		return err
	}
	// hoverRange takes a Range as the position when a selection is active;
	// otherwise the plain cursor position gives the usual hover.
	var position any = lspcmd.CoordToPos(cmd.Cursor.Content)
	if rng, ok := c.sel.Get(cmd.URI); ok {
		position = rng
	}
	params := struct {
		TextDocument semanticapi.TextDocumentIdentifier `json:"textDocument"`
		Position     any                                `json:"position"`
	}{TextDocument: docParams(cmd), Position: position}
	res, err := execRequest[*struct {
		Contents struct {
			Value string `json:"value"`
		} `json:"contents"`
	}](ctx, c.lsp, "textDocument/hover", params)
	if err != nil {
		return err
	}
	if res == nil || strings.TrimSpace(res.Contents.Value) == "" {
		_, _ = c.notify.Notify(browserapi.LevelInfo, "No type information for the selection")
		return nil
	}
	return showMarkdown(c.wm, c.parser, c.interrupt, res.Contents.Value)
}

func (c *hoverRangeCmd) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

// workspaceSymbolCmd runs rust-analyzer's workspace/symbol with the
// experimental scope/kind filters (types only, workspace or workspace +
// dependencies). The query and an optional scope keyword come from the
// command args: `rust symbols <query> [deps]`.
type workspaceSymbolCmd struct {
	pickDeps
	lsp semanticapi.LSP
}

var _ textapi.CommandHandler = (*workspaceSymbolCmd)(nil)

func (c *workspaceSymbolCmd) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	win, err := c.wm.Focus()
	if err != nil {
		return err
	}
	args := append([]string(nil), cmd.Args...)
	scope := "workspace"
	if n := len(args); n > 0 && (args[n-1] == "deps" || args[n-1] == "dependencies") {
		scope = "workspaceAndDependencies"
		args = args[:n-1]
	}
	query := strings.TrimSpace(strings.Join(args, " "))
	if query == "" {
		return fmt.Errorf("usage: rust symbols <query> [deps]")
	}
	params := struct {
		Query       string `json:"query"`
		SearchScope string `json:"searchScope"`
		SearchKind  string `json:"searchKind"`
	}{Query: query, SearchScope: scope, SearchKind: "onlyTypes"}
	symbols, err := execRequest[[]semanticapi.SymbolInformation](ctx, c.lsp, "workspace/symbol", params)
	if err != nil {
		return err
	}
	entries := make([]pickEntry, len(symbols))
	for i, s := range symbols {
		entries[i] = pickEntry{
			loc: s.Location,
			display: fmt.Sprintf("%s\t%s:%d", s.Name,
				c.relPath(cmd.URI, s.Location.URI), s.Location.Range.Start.Line+1),
		}
	}
	return c.present(ctx, win, cmd.URI, entries, "No matching workspace symbols")
}

func (c *workspaceSymbolCmd) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

// trimFileURI drops the file:// prefix so viewer lines show a path.
func trimFileURI(uri string) string {
	return strings.TrimPrefix(uri, "file://")
}
