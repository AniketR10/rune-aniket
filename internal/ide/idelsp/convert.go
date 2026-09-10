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

package idelsp

import (
	"encoding/json"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
)

type lspPosition struct {
	Line      uint32 `json:"line"`
	Character uint32 `json:"character"`
}

type lspRange struct {
	Start lspPosition `json:"start"`
	End   lspPosition `json:"end"`
}

type lspTextEdit struct {
	Range   lspRange `json:"range"`
	NewText string   `json:"newText"`
}

type lspMarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type lspCompletionItem struct {
	Label            string          `json:"label"`
	Kind             int             `json:"kind,omitempty"`
	Detail           string          `json:"detail,omitempty"`
	Documentation    json.RawMessage `json:"documentation,omitempty"`
	InsertText       string          `json:"insertText,omitempty"`
	InsertTextFormat int             `json:"insertTextFormat,omitempty"`
	TextEdit         *lspTextEdit    `json:"textEdit,omitempty"`
}

type lspCompletionList struct {
	IsIncomplete bool                `json:"isIncomplete"`
	Items        []lspCompletionItem `json:"items"`
}

type lspSignatureHelp struct {
	Signatures      []lspSignatureInformation `json:"signatures"`
	ActiveSignature uint32                    `json:"activeSignature"`
	ActiveParameter uint32                    `json:"activeParameter"`
}

type lspSignatureInformation struct {
	Label         string                    `json:"label"`
	Documentation json.RawMessage           `json:"documentation,omitempty"`
	Parameters    []lspParameterInformation `json:"parameters,omitempty"`
}

type lspParameterInformation struct {
	Label         json.RawMessage `json:"label"`
	Documentation json.RawMessage `json:"documentation,omitempty"`
}

type lspTextDocumentEdit struct {
	TextDocument struct {
		URI     string `json:"uri"`
		Version *int   `json:"version,omitempty"`
	} `json:"textDocument"`
	Edits []lspTextEdit `json:"edits"`
}

type lspWorkspaceEdit struct {
	Changes         map[string][]lspTextEdit `json:"changes,omitempty"`
	DocumentChanges []lspTextDocumentEdit    `json:"documentChanges,omitempty"`
}

type lspCodeAction struct {
	Title   string            `json:"title"`
	Kind    string            `json:"kind,omitempty"`
	Edit    *lspWorkspaceEdit `json:"edit,omitempty"`
	Command *lspCommand       `json:"command,omitempty"`
	// Group is rust-analyzer's codeActionGroup extension field. Actions
	// sharing a group are collapsed under one lightbulb entry. Absent for
	// servers that do not implement the extension.
	Group string `json:"group,omitempty"`
}

type lspCommand struct {
	Title     string            `json:"title"`
	Command   string            `json:"command"`
	Arguments []json.RawMessage `json:"arguments,omitempty"`
}

func lspTextEditToSemantic(
	e lspTextEdit,
) semanticapi.TextEdit {
	return semanticapi.TextEdit{
		Range: semanticapi.Range{
			Start: semanticapi.Position{
				Line:      e.Range.Start.Line,
				Character: e.Range.Start.Character,
			},
			End: semanticapi.Position{
				Line:      e.Range.End.Line,
				Character: e.Range.End.Character,
			},
		},
		NewText: e.NewText,
	}
}

func lspMarkupToSemantic(
	m lspMarkupContent,
) semanticapi.MarkupContent {
	kind := semanticapi.MarkupKindPlainText
	if m.Kind == "markdown" {
		kind = semanticapi.MarkupKindMarkdown
	}
	return semanticapi.MarkupContent{
		Kind: kind, Value: m.Value,
	}
}

func lspWorkspaceEditToSemantic(
	e *lspWorkspaceEdit,
) *semanticapi.WorkspaceEdit {
	if e == nil {
		return nil
	}
	ret := &semanticapi.WorkspaceEdit{
		Changes: make(
			map[string][]semanticapi.TextEdit,
		),
	}
	for uri, edits := range e.Changes {
		for _, te := range edits {
			ret.Changes[uri] = append(
				ret.Changes[uri],
				lspTextEditToSemantic(te),
			)
		}
	}
	for _, dc := range e.DocumentChanges {
		uri := dc.TextDocument.URI
		for _, te := range dc.Edits {
			ret.Changes[uri] = append(
				ret.Changes[uri],
				lspTextEditToSemantic(te),
			)
		}
	}
	return ret
}

// lspCodeActionToSemantic converts a decoded LSP code action, including
// rust-analyzer's codeActionGroup "group" field, into the SDK type.
func lspCodeActionToSemantic(a lspCodeAction) semanticapi.CodeAction {
	action := semanticapi.CodeAction{
		Title: a.Title,
		Kind:  semanticapi.CodeActionKind(a.Kind),
		Edit:  lspWorkspaceEditToSemantic(a.Edit),
		Group: a.Group,
	}
	if a.Command != nil {
		cmd := &semanticapi.Command{
			Title:   a.Command.Title,
			Command: a.Command.Command,
		}
		cmd.Arguments = append(cmd.Arguments, a.Command.Arguments...)
		action.Command = cmd
	}
	return action
}

func lspCompletionItemToSemantic(
	item lspCompletionItem,
) semanticapi.CompletionItem {
	ret := semanticapi.CompletionItem{
		Label: item.Label,
		Kind: semanticapi.CompletionItemKind(
			item.Kind,
		),
		Detail:     item.Detail,
		InsertText: item.InsertText,
		InsertTextFormat: semanticapi.InsertTextFormat(
			item.InsertTextFormat,
		),
	}
	if item.Documentation != nil {
		var mc lspMarkupContent
		if err := json.Unmarshal(
			item.Documentation, &mc,
		); err == nil && mc.Value != "" {
			v := lspMarkupToSemantic(mc)
			ret.Documentation = &v
		} else {
			var s string
			if err := json.Unmarshal(
				item.Documentation, &s,
			); err == nil && s != "" {
				ret.Documentation = &semanticapi.MarkupContent{
					Kind:  semanticapi.MarkupKindPlainText,
					Value: s,
				}
			}
		}
	}
	if item.TextEdit != nil {
		te := lspTextEditToSemantic(*item.TextEdit)
		ret.TextEdit = &te
	}
	return ret
}

func lspSignatureHelpToSemantic(
	sh *lspSignatureHelp,
) *semanticapi.SignatureHelp {
	if sh == nil {
		return nil
	}
	ret := &semanticapi.SignatureHelp{
		ActiveSignature: sh.ActiveSignature,
		ActiveParameter: sh.ActiveParameter,
	}
	for _, sig := range sh.Signatures {
		si := semanticapi.SignatureInformation{
			Label: sig.Label,
		}
		if sig.Documentation != nil {
			var mc lspMarkupContent
			if err := json.Unmarshal(
				sig.Documentation, &mc,
			); err == nil && mc.Value != "" {
				v := lspMarkupToSemantic(mc)
				si.Documentation = &v
			}
		}
		for _, p := range sig.Parameters {
			pi := semanticapi.ParameterInformation{}
			var labelStr string
			if err := json.Unmarshal(
				p.Label, &labelStr,
			); err == nil {
				pi.Label = labelStr
			} else {
				var labelArr [2]int
				if err := json.Unmarshal(
					p.Label, &labelArr,
				); err == nil {
					pi.Label = sig.Label[labelArr[0]:labelArr[1]]
				}
			}
			if p.Documentation != nil {
				var mc lspMarkupContent
				if err := json.Unmarshal(
					p.Documentation, &mc,
				); err == nil && mc.Value != "" {
					v := lspMarkupToSemantic(mc)
					pi.Documentation = &v
				}
			}
			si.Parameters = append(
				si.Parameters, pi,
			)
		}
		ret.Signatures = append(ret.Signatures, si)
	}
	return ret
}
