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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/ide/idelsp/lspcmd"
)

// navCmd opens a Location returned by a rust-analyzer navigation
// extension (parentModule, openCargoToml) and moves the cursor there.
type navCmd struct {
	lsp    semanticapi.LSP
	editor textapi.Editor
	wm     browserapi.WindowManager
	opener browserapi.ResourceOpener
	notify browserapi.Notifications
	method string
	posArg bool
	// notFound is reported when the server returns no location.
	notFound string
}

var _ textapi.CommandHandler = (*navCmd)(nil)

func (c *navCmd) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	if err := requireFile(cmd); err != nil {
		return err
	}
	var params any = struct {
		TextDocument semanticapi.TextDocumentIdentifier `json:"textDocument"`
	}{TextDocument: docParams(cmd)}
	if c.posArg {
		params = posParams(cmd)
	}
	loc, err := execRequest[semanticapi.LocationResult](ctx, c.lsp, c.method, params)
	if err != nil {
		return err
	}
	entries := locationEntries(loc)
	if len(entries) == 0 {
		_, _ = c.notify.Notify(browserapi.LevelInfo, "%s", c.notFound)
		return nil
	}
	return c.open(cmd.URI, entries[0])
}

func (c *navCmd) open(base workspaceapi.URI, loc semanticapi.Location) error {
	return openLocation(c.editor, c.wm, c.opener, base, loc)
}

// openLocation opens the file at loc, focuses it, and moves the cursor to
// the start of loc's range. It is shared by the navigation and
// workspace-symbol commands. base is any URI of the workspace, used to
// rebase the server's file:// location onto the workspace scheme.
func openLocation(
	editor textapi.Editor, wm browserapi.WindowManager,
	opener browserapi.ResourceOpener, base workspaceapi.URI,
	loc semanticapi.Location,
) error {
	uri, err := lspcmd.LspToURI(base, loc.URI)
	if err != nil {
		return err
	}
	if _, err := opener.Open(uri); err != nil {
		return fmt.Errorf("open %s: %w", uri.Name(), err)
	}
	w, err := wm.Focus()
	if err != nil {
		return err
	}
	h, err := editor.Editor(uri)
	if err != nil {
		return err
	}
	if err := wm.SetWindowContent(w, h); err != nil && !errors.Is(err, browserapi.ErrTabNotFree) {
		return err
	}
	target := lspcmd.PosToCoord(loc.Range.Start)
	if err := editor.SetCursor(h, target); err != nil {
		if cur, curErr := editor.Cursor(h); curErr == nil && cur == target {
			return nil
		}
		return err
	}
	return nil
}

func (c *navCmd) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

// locationEntries flattens a LocationResult (single, list, or links) into
// a slice of Locations so callers can take the first.
func locationEntries(r semanticapi.LocationResult) []semanticapi.Location {
	var out []semanticapi.Location
	if r.Location != nil {
		out = append(out, *r.Location)
	}
	out = append(out, r.Locations...)
	for _, ll := range r.LocationLinks {
		out = append(out, semanticapi.Location{URI: ll.TargetURI, Range: ll.TargetSelectionRange})
	}
	return out
}

// externalDocsCmd requests the docs.rs (or local) documentation URL for
// the symbol at the cursor and reports it. Rune has no in-app browser for
// external URLs, so the URL is surfaced via a notification.
type externalDocsCmd struct {
	lsp    semanticapi.LSP
	notify browserapi.Notifications
}

var _ textapi.CommandHandler = (*externalDocsCmd)(nil)

func (c *externalDocsCmd) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	if err := requireFile(cmd); err != nil {
		return err
	}
	raw, err := c.lsp.ExecuteRequest(ctx, semanticapi.ExecuteRequestParams{
		Method: "experimental/externalDocs",
		Params: mustMarshal(posParams(cmd)),
	})
	if err != nil {
		return fmt.Errorf("experimental/externalDocs: %w", err)
	}
	url := parseExternalDocs(raw)
	if url == "" {
		_, _ = c.notify.Notify(browserapi.LevelInfo, "No external documentation for the symbol at the cursor")
		return nil
	}
	_, _ = c.notify.Notify(browserapi.LevelInfo, "Docs: %s", url)
	return nil
}

func (c *externalDocsCmd) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

// parseExternalDocs decodes externalDocs' result, which is either a bare
// string (older servers) or a {web?, local?} object, preferring the web
// URL.
func parseExternalDocs(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var obj struct {
		Web   string `json:"web"`
		Local string `json:"local"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	if obj.Web != "" {
		return obj.Web
	}
	return obj.Local
}

func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}
