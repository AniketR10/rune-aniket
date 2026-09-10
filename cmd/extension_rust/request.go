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

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"google.golang.org/grpc/status"
	"unstable.build/rune/internal/ide/idelsp/lspcmd"
)

// posParams builds a TextDocumentPositionParams from a command's URI and
// cursor. rust-analyzer's position-based extensions (viewHir, moveItem,
// onEnter, ...) all take this shape.
func posParams(cmd textapi.Command) semanticapi.TextDocumentPositionParams {
	return semanticapi.TextDocumentPositionParams{
		TextDocument: lspcmd.TextDocID(cmd.URI),
		Position:     lspcmd.CoordToPos(cmd.Cursor.Content),
	}
}

// docParams builds a TextDocumentIdentifier from a command's URI.
func docParams(cmd textapi.Command) semanticapi.TextDocumentIdentifier {
	return lspcmd.TextDocID(cmd.URI)
}

// execRequest forwards method with params to the language server and
// unmarshals its raw JSON result into T. A "null" result leaves the zero
// value of T unchanged.
func execRequest[T any](
	ctx context.Context, lsp semanticapi.LSP, method string, params any,
) (T, error) {
	var zero T
	var raw json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return zero, fmt.Errorf("marshal %s params: %w", method, err)
		}
		raw = data
	}
	result, err := lsp.ExecuteRequest(ctx, semanticapi.ExecuteRequestParams{
		Method: method,
		Params: raw,
	})
	if err != nil {
		return zero, fmt.Errorf("%s: %s", method, lspErrorMessage(err))
	}
	if len(result) == 0 || string(result) == "null" {
		return zero, nil
	}
	var out T
	if err := json.Unmarshal(result, &out); err != nil {
		return zero, fmt.Errorf("decode %s result: %w", method, err)
	}
	return out, nil
}

// lspErrorMessage extracts the server-reported message from a request
// error. Extension requests travel over gRPC, whose status rendering
// ("rpc error: code = Unknown desc = ...") buries the rust-analyzer
// error the user needs to see (e.g. "request handler panicked: ...").
func lspErrorMessage(err error) string {
	if s, ok := status.FromError(err); ok {
		return s.Message()
	}
	return err.Error()
}

// sendNotification forwards a notification method with params to the
// language server. A nil params sends no payload.
func sendNotification(
	ctx context.Context, lsp semanticapi.LSP, method string, params any,
) error {
	var raw json.RawMessage
	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			return fmt.Errorf("marshal %s params: %w", method, err)
		}
		raw = data
	}
	if err := lsp.SendNotification(ctx, semanticapi.NotificationParams{
		Method: method,
		Params: raw,
	}); err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	return nil
}

// snippetTextEdit is rust-analyzer's experimental TextEdit variant whose
// newText may embed LSP snippet tab stops ($0, ${1}, ${1:name}). Rune
// applies edits verbatim, so the markers must be stripped before use.
type snippetTextEdit struct {
	Range   semanticapi.Range `json:"range"`
	NewText string            `json:"newText"`
	// InsertTextFormat 2 marks a snippet; 1 (or absent) is plain text.
	InsertTextFormat int `json:"insertTextFormat,omitempty"`
}

// snippetTabStop matches ${0}, ${1:name}, and bare $0/$1 tab stops.
var snippetTabStop = regexp.MustCompile(`\$\{(\d+)(?::([^}]*))?\}|\$(\d+)`)

// stripSnippetMarkers removes LSP snippet tab stops from s, keeping the
// placeholder name of ${n:name} forms so a rename target stays readable.
func stripSnippetMarkers(s string) string {
	return snippetTabStop.ReplaceAllString(s, "$2")
}

// toTextEdits converts snippet edits to plain TextEdits, stripping snippet
// markers from any edit flagged as a snippet.
func toTextEdits(edits []snippetTextEdit) []semanticapi.TextEdit {
	out := make([]semanticapi.TextEdit, len(edits))
	for i, e := range edits {
		text := e.NewText
		if e.InsertTextFormat == 2 {
			text = stripSnippetMarkers(text)
		}
		out[i] = semanticapi.TextEdit{Range: e.Range, NewText: text}
	}
	return out
}
