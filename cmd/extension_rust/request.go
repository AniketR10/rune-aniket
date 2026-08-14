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
	"fmt"
	"regexp"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"google.golang.org/grpc/status"
	"unstable.build/go-tui/ide/idelsp/lspcmd"
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
