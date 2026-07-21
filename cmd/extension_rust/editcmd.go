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
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/ide/idelsp/lspcmd"
)

// editDeps are the shared dependencies of the edit-applying subcommands.
type editDeps struct {
	lsp    semanticapi.LSP
	editor textapi.Editor
	opener browserapi.ResourceOpener
	notify browserapi.Notifications
	sel    *lspcmd.SelectionTracker
}

// cursorRange returns the current selection for the command's URI, or a
// collapsed range at the cursor when nothing is selected. The selection is
// clamped to the buffer so out-of-line columns do not reach the server.
func (d editDeps) cursorRange(cmd textapi.Command) semanticapi.Range {
	if rng, ok := d.sel.Get(cmd.URI); ok {
		return lspcmd.ClampRange(rng, d.editor, cmd.Resource)
	}
	pos := lspcmd.CoordToPos(cmd.Cursor.Content)
	return semanticapi.Range{Start: pos, End: pos}
}

// applyEdits routes plain text edits for the command's document through
// the editor, opening the file if needed.
func (d editDeps) applyEdits(
	ctx context.Context, cmd textapi.Command, edits []semanticapi.TextEdit,
) error {
	if len(edits) == 0 {
		return nil
	}
	return lspcmd.ApplyEditsForURI(ctx, d.editor, d.opener, cmd.URI, lspcmd.URIToLSP(cmd.URI), edits)
}

// joinLinesCmd requests experimental/joinLines over the selection (or the
// cursor line) and applies the returned TextEdits.
type joinLinesCmd struct{ editDeps }

var _ textapi.CommandHandler = (*joinLinesCmd)(nil)

func (c *joinLinesCmd) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	if err := requireFile(cmd); err != nil {
		return err
	}
	params := struct {
		TextDocument semanticapi.TextDocumentIdentifier `json:"textDocument"`
		Ranges       []semanticapi.Range                `json:"ranges"`
	}{TextDocument: docParams(cmd), Ranges: []semanticapi.Range{c.cursorRange(cmd)}}
	edits, err := execRequest[[]semanticapi.TextEdit](ctx, c.lsp, "experimental/joinLines", params)
	if err != nil {
		return err
	}
	if len(edits) == 0 {
		_, _ = c.notify.Notify(browserapi.LevelInfo, "Nothing to join at the cursor")
		return nil
	}
	return c.applyEdits(ctx, cmd, edits)
}

func (c *joinLinesCmd) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

// matchingBraceCmd requests experimental/matchingBrace at the cursor and
// moves the cursor to the matching brace position.
type matchingBraceCmd struct{ editDeps }

var _ textapi.CommandHandler = (*matchingBraceCmd)(nil)

func (c *matchingBraceCmd) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	if err := requireFile(cmd); err != nil {
		return err
	}
	params := struct {
		TextDocument semanticapi.TextDocumentIdentifier `json:"textDocument"`
		Positions    []semanticapi.Position             `json:"positions"`
	}{TextDocument: docParams(cmd), Positions: []semanticapi.Position{lspcmd.CoordToPos(cmd.Cursor.Content)}}
	positions, err := execRequest[[]semanticapi.Position](ctx, c.lsp, "experimental/matchingBrace", params)
	if err != nil {
		return err
	}
	if len(positions) == 0 {
		_, _ = c.notify.Notify(browserapi.LevelInfo, "No matching brace at the cursor")
		return nil
	}
	h, err := c.editor.Editor(cmd.URI)
	if err != nil {
		return err
	}
	return c.editor.SetCursor(h, lspcmd.PosToCoord(positions[0]))
}

func (c *matchingBraceCmd) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

// snippetEditCmd requests a method returning SnippetTextEdits (onEnter,
// moveItem), strips snippet markers, and applies the result. moveItem
// carries a direction; onEnter leaves it empty.
type snippetEditCmd struct {
	editDeps
	method    string
	direction string
	emptyMsg  string
}

var _ textapi.CommandHandler = (*snippetEditCmd)(nil)

func (c *snippetEditCmd) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	if err := requireFile(cmd); err != nil {
		return err
	}
	var params any
	if c.direction != "" {
		params = struct {
			TextDocument semanticapi.TextDocumentIdentifier `json:"textDocument"`
			Range        semanticapi.Range                  `json:"range"`
			Direction    string                             `json:"direction"`
		}{TextDocument: docParams(cmd), Range: c.cursorRange(cmd), Direction: c.direction}
	} else {
		params = posParams(cmd)
	}
	edits, err := execRequest[[]snippetTextEdit](ctx, c.lsp, c.method, params)
	if err != nil {
		return err
	}
	if len(edits) == 0 {
		_, _ = c.notify.Notify(browserapi.LevelInfo, "%s", c.emptyMsg)
		return nil
	}
	return c.applyEdits(ctx, cmd, toTextEdits(edits))
}

func (c *snippetEditCmd) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

// ssrCmd runs a Structural Search Replace query from the command args and
// applies the resulting WorkspaceEdit. The query uses rust-analyzer's
// `pattern ==>> replacement` syntax, e.g. `foo($a) ==>> bar($a)`.
type ssrCmd struct{ editDeps }

var _ textapi.CommandHandler = (*ssrCmd)(nil)

func (c *ssrCmd) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	if err := requireFile(cmd); err != nil {
		return err
	}
	query := strings.TrimSpace(strings.Join(cmd.Args, " "))
	if query == "" {
		return fmt.Errorf("usage: rust ssr \"<pattern> ==>> <replacement>\"")
	}
	params := struct {
		Query        string                             `json:"query"`
		ParseOnly    bool                               `json:"parseOnly"`
		TextDocument semanticapi.TextDocumentIdentifier `json:"textDocument"`
		Position     semanticapi.Position               `json:"position"`
		Selections   []semanticapi.Range                `json:"selections"`
	}{
		Query:        query,
		TextDocument: docParams(cmd),
		Position:     lspcmd.CoordToPos(cmd.Cursor.Content),
		Selections:   []semanticapi.Range{},
	}
	edit, err := execRequest[*semanticapi.WorkspaceEdit](ctx, c.lsp, "experimental/ssr", params)
	if err != nil {
		return err
	}
	if edit == nil {
		_, _ = c.notify.Notify(browserapi.LevelInfo, "SSR query matched nothing")
		return nil
	}
	return lspcmd.ApplyWorkspaceEdit(ctx, c.editor, c.opener, cmd.URI, edit, nil)
}

func (c *ssrCmd) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}
