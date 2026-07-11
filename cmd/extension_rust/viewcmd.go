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
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// viewResult builds the text a viewer command displays from the raw
// server result. It runs after the request succeeds so a command can
// project a structured result into readable lines.
type viewResult func(ctx context.Context, cmd textapi.Command) (string, error)

// viewCmd requests text from rust-analyzer and shows it in a floating,
// scrollable read-only viewer. The empty-output case reports a hint
// instead of floating an empty window.
type viewCmd struct {
	lsp      semanticapi.LSP
	wm       browserapi.WindowManager
	notify   browserapi.Notifications
	produce  viewResult
	emptyMsg string
}

var _ textapi.CommandHandler = (*viewCmd)(nil)

func (c *viewCmd) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	text, err := c.produce(ctx, cmd)
	if err != nil {
		return err
	}
	if text == "" {
		_, _ = c.notify.Notify(browserapi.LevelInfo, "%s", c.emptyMsg)
		return nil
	}
	if _, err := c.wm.Floating(newTextView(text), browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	}); err != nil {
		return fmt.Errorf("show viewer: %w", err)
	}
	return nil
}

func (c *viewCmd) Complete(_ context.Context, _ string, _ []string) (
	iterator.Iterator[string], error,
) {
	return iterator.Empty[string](), nil
}

// requireFile returns an error when no file is open, matching the guard
// the assist commands use.
func requireFile(cmd textapi.Command) error {
	if cmd.Resource == nil {
		return fmt.Errorf("no file open; open a file first")
	}
	return nil
}

// stringViewCmd wires a viewer command whose server method returns a bare
// JSON string (or an object rust-analyzer renders as one).
func stringViewCmd(
	lsp semanticapi.LSP, wm browserapi.WindowManager,
	notify browserapi.Notifications, produce viewResult, emptyMsg string,
) *viewCmd {
	return &viewCmd{lsp: lsp, wm: wm, notify: notify, produce: produce, emptyMsg: emptyMsg}
}

// posTextView requests a position-based rust-analyzer method returning a
// plain string (viewHir, viewMir, interpretFunction).
func posTextView(lsp semanticapi.LSP, method string) viewResult {
	return func(ctx context.Context, cmd textapi.Command) (string, error) {
		if err := requireFile(cmd); err != nil {
			return "", err
		}
		return execRequest[string](ctx, lsp, method, posParams(cmd))
	}
}

// docTextView requests a document-based rust-analyzer method that takes a
// {textDocument} object and returns a plain string.
func docTextView(lsp semanticapi.LSP, method string) viewResult {
	return func(ctx context.Context, cmd textapi.Command) (string, error) {
		if err := requireFile(cmd); err != nil {
			return "", err
		}
		params := struct {
			TextDocument semanticapi.TextDocumentIdentifier `json:"textDocument"`
		}{TextDocument: docParams(cmd)}
		return execRequest[string](ctx, lsp, method, params)
	}
}

// fileTextView requests viewFileText, whose params are a bare
// TextDocumentIdentifier rather than a wrapping object.
func fileTextView(lsp semanticapi.LSP) viewResult {
	return func(ctx context.Context, cmd textapi.Command) (string, error) {
		if err := requireFile(cmd); err != nil {
			return "", err
		}
		return execRequest[string](ctx, lsp, "rust-analyzer/viewFileText", docParams(cmd))
	}
}

// analyzerStatusView requests analyzerStatus with the current document so
// rust-analyzer reports crate-specific status.
func analyzerStatusView(lsp semanticapi.LSP) viewResult {
	return func(ctx context.Context, cmd textapi.Command) (string, error) {
		params := struct {
			TextDocument *semanticapi.TextDocumentIdentifier `json:"textDocument,omitempty"`
		}{}
		if cmd.Resource != nil {
			doc := docParams(cmd)
			params.TextDocument = &doc
		}
		return execRequest[string](ctx, lsp, "rust-analyzer/analyzerStatus", params)
	}
}

// expandMacroView requests expandMacro at the cursor and renders the
// expansion, prefixed with the macro name.
func expandMacroView(lsp semanticapi.LSP) viewResult {
	return func(ctx context.Context, cmd textapi.Command) (string, error) {
		if err := requireFile(cmd); err != nil {
			return "", err
		}
		params := struct {
			TextDocument semanticapi.TextDocumentIdentifier `json:"textDocument"`
			Position     semanticapi.Position               `json:"position"`
		}{TextDocument: docParams(cmd), Position: posParams(cmd).Position}
		res, err := execRequest[*struct {
			Name      string `json:"name"`
			Expansion string `json:"expansion"`
		}](ctx, lsp, "rust-analyzer/expandMacro", params)
		if err != nil || res == nil {
			return "", err
		}
		if res.Name == "" {
			return res.Expansion, nil
		}
		return fmt.Sprintf("// %s\n%s", res.Name, res.Expansion), nil
	}
}

// noParamsView requests a method that takes no params and returns a plain
// string (memoryUsage, viewCrateGraph, ...).
func noParamsView(lsp semanticapi.LSP, method string, params any) viewResult {
	return func(ctx context.Context, cmd textapi.Command) (string, error) {
		return execRequest[string](ctx, lsp, method, params)
	}
}

// dependenciesView requests fetchDependencyList and renders one crate per
// line as "name version".
func dependenciesView(lsp semanticapi.LSP) viewResult {
	return func(ctx context.Context, cmd textapi.Command) (string, error) {
		res, err := execRequest[struct {
			Crates []struct {
				Name    string `json:"name"`
				Version string `json:"version"`
				Path    string `json:"path"`
			} `json:"crates"`
		}](ctx, lsp, "rust-analyzer/fetchDependencyList", struct{}{})
		if err != nil {
			return "", err
		}
		var b []byte
		for _, c := range res.Crates {
			line := c.Name
			if c.Version != "" {
				line += " " + c.Version
			}
			b = append(b, line...)
			b = append(b, '\n')
		}
		return string(b), nil
	}
}

// runnablesView requests runnables at the cursor and lists their labels.
// Rune does not run cargo targets from this surface, so the labels are
// informational only.
func runnablesView(lsp semanticapi.LSP) viewResult {
	return func(ctx context.Context, cmd textapi.Command) (string, error) {
		if err := requireFile(cmd); err != nil {
			return "", err
		}
		params := struct {
			TextDocument semanticapi.TextDocumentIdentifier `json:"textDocument"`
			Position     *semanticapi.Position              `json:"position,omitempty"`
		}{TextDocument: docParams(cmd)}
		pos := posParams(cmd).Position
		params.Position = &pos
		res, err := execRequest[[]struct {
			Label string `json:"label"`
		}](ctx, lsp, "experimental/runnables", params)
		if err != nil {
			return "", err
		}
		var b []byte
		for _, r := range res {
			b = append(b, r.Label...)
			b = append(b, '\n')
		}
		return string(b), nil
	}
}

// relatedTestsView requests relatedTests at the cursor and lists the test
// runnable labels.
func relatedTestsView(lsp semanticapi.LSP) viewResult {
	return func(ctx context.Context, cmd textapi.Command) (string, error) {
		if err := requireFile(cmd); err != nil {
			return "", err
		}
		res, err := execRequest[[]struct {
			Runnable struct {
				Label string `json:"label"`
			} `json:"runnable"`
		}](ctx, lsp, "rust-analyzer/relatedTests", posParams(cmd))
		if err != nil {
			return "", err
		}
		var b []byte
		for _, r := range res {
			b = append(b, r.Runnable.Label...)
			b = append(b, '\n')
		}
		return string(b), nil
	}
}

// evalPredicateView asks rust-analyzer to evaluate a trait predicate (e.g.
// `T: Clone`) in the context of the item at the cursor, and renders the
// resulting status and message. The predicate text comes from the command
// arguments: `rust eval-predicate T: Clone`.
func evalPredicateView(lsp semanticapi.LSP) viewResult {
	return func(ctx context.Context, cmd textapi.Command) (string, error) {
		if err := requireFile(cmd); err != nil {
			return "", err
		}
		text := strings.TrimSpace(strings.Join(cmd.Args, " "))
		if text == "" {
			return "", fmt.Errorf("usage: rust eval-predicate <predicate>")
		}
		params := struct {
			Text         string                             `json:"text"`
			TextDocument semanticapi.TextDocumentIdentifier `json:"textDocument"`
			Position     semanticapi.Position               `json:"position"`
		}{Text: text, TextDocument: docParams(cmd), Position: posParams(cmd).Position}
		res, err := execRequest[*struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		}](ctx, lsp, "rust-analyzer/evaluatePredicate", params)
		if err != nil || res == nil {
			return "", err
		}
		out := res.Status
		if res.Message != "" {
			out += "\n\n" + res.Message
		}
		return out, nil
	}
}

// recursiveMemoryLayoutView requests viewRecursiveMemoryLayout at the
// cursor and renders each node's name/size/offset.
func recursiveMemoryLayoutView(lsp semanticapi.LSP) viewResult {
	return func(ctx context.Context, cmd textapi.Command) (string, error) {
		if err := requireFile(cmd); err != nil {
			return "", err
		}
		res, err := execRequest[*struct {
			Nodes []struct {
				Item   string `json:"itemName"`
				Size   uint64 `json:"size"`
				Offset uint64 `json:"offset"`
			} `json:"nodes"`
		}](ctx, lsp, "rust-analyzer/viewRecursiveMemoryLayout", posParams(cmd))
		if err != nil || res == nil {
			return "", err
		}
		var b []byte
		for _, n := range res.Nodes {
			b = append(b, fmt.Sprintf("%s\tsize=%d\toffset=%d\n", n.Item, n.Size, n.Offset)...)
		}
		return string(b), nil
	}
}

// failedObligationsView requests the failed trait obligations for the item
// at the cursor.
func failedObligationsView(lsp semanticapi.LSP) viewResult {
	return func(ctx context.Context, cmd textapi.Command) (string, error) {
		if err := requireFile(cmd); err != nil {
			return "", err
		}
		return execRequest[string](ctx, lsp, "rust-analyzer/getFailedObligations", posParams(cmd))
	}
}

// diagnosticsView pulls the diagnostics for the current file and renders
// each as "severity[code] line:col message". When rust-analyzer attaches a
// rustc-rendered ANSI block (data.rendered, from the colorDiagnosticOutput
// capability), that full compiler output is appended verbatim.
func diagnosticsView(lsp semanticapi.LSP) viewResult {
	return func(ctx context.Context, cmd textapi.Command) (string, error) {
		if err := requireFile(cmd); err != nil {
			return "", err
		}
		report, err := lsp.Diagnostic(ctx, semanticapi.DocumentDiagnosticParams{
			TextDocument: docParams(cmd),
		})
		if err != nil {
			return "", fmt.Errorf("document diagnostics: %w", err)
		}
		var b strings.Builder
		for _, d := range report.Items {
			fmt.Fprintf(&b, "%s%s %d:%d %s\n",
				diagnosticSeverityLabel(d.Severity), diagnosticCode(d),
				d.Range.Start.Line+1, d.Range.Start.Character+1, d.Message)
			if rendered := diagnosticRendered(d.Data); rendered != "" {
				b.WriteString(rendered)
				if !strings.HasSuffix(rendered, "\n") {
					b.WriteByte('\n')
				}
			}
		}
		return b.String(), nil
	}
}

func diagnosticSeverityLabel(s semanticapi.DiagnosticSeverity) string {
	switch s {
	case semanticapi.DiagnosticSeverityError:
		return "error"
	case semanticapi.DiagnosticSeverityWarning:
		return "warning"
	case semanticapi.DiagnosticSeverityInformation:
		return "info"
	case semanticapi.DiagnosticSeverityHint:
		return "hint"
	default:
		return "diagnostic"
	}
}

func diagnosticCode(d semanticapi.Diagnostic) string {
	if d.CodeIsInt {
		return fmt.Sprintf("[%d]", d.CodeInt)
	}
	if d.Code != "" {
		return "[" + d.Code + "]"
	}
	return ""
}

// diagnosticRendered extracts rust-analyzer's rustc-rendered ANSI output
// from a diagnostic's data field, when present.
func diagnosticRendered(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}
	var v struct {
		Rendered string `json:"rendered"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return ""
	}
	return v.Rendered
}
