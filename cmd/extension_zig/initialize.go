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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
)

// buildOnSaveOptions carries the optional build-on-save configuration
// forwarded to zls. A nil Enable leaves zls's default in place: zls
// auto-enables build-on-save when build.zig declares a "check" step.
type buildOnSaveOptions struct {
	Enable *bool
	Args   []string
}

func zlsCommand(bin, logLevel string) string {
	if logLevel == "" {
		logLevel = "info"
	}
	return strings.Join([]string{
		bin, "--enable-stderr-logs", "--log-level", logLevel,
	}, " ")
}

// zlsInitializeParams builds the InitializeParams for zls. The
// langID/command keys steer the host LSP shim; every other key is
// forwarded verbatim as zls's config via initializationOptions.
//
// Two zls-specific requirements shape the params:
//   - zls only registers workspaces (the scope of workspace/symbol and
//     build-on-save) from workspaceFolders; rootUri alone is ignored.
//   - zls only pushes diagnostics when the client advertises the
//     textDocument.publishDiagnostics capability.
func zlsInitializeParams(
	rootURI, rootName, command, zigExePath, logLevel string,
	bos buildOnSaveOptions,
) (semanticapi.InitializeParams, error) {
	initOptions := map[string]any{
		"langID":  "zig",
		"command": zlsCommand(command, logLevel),
		// Plain-text completions: Rune's editor inserts completion text
		// verbatim, so snippet placeholders must never reach the buffer.
		"enable_snippets":                        false,
		"enable_argument_placeholders":           false,
		"inlay_hints_show_variable_type_hints":   true,
		"inlay_hints_show_parameter_name":        true,
		"inlay_hints_show_builtin":               true,
		"inlay_hints_hide_redundant_param_names": true,
	}
	if zigExePath != "" {
		initOptions["zig_exe_path"] = zigExePath
	}
	if bos.Enable != nil {
		initOptions["enable_build_on_save"] = *bos.Enable
	}
	if len(bos.Args) > 0 {
		initOptions["build_on_save_args"] = bos.Args
	}

	initOptionsData, err := json.Marshal(initOptions)
	if err != nil {
		return semanticapi.InitializeParams{}, fmt.Errorf("marshal initialize options: %w", err)
	}

	// Only the surface zls 0.16 implements is advertised: no
	// implementation, rangeFormatting, codeLens, or call/type hierarchy.
	capabilities := map[string]any{
		"textDocument": map[string]any{
			"hover": map[string]any{
				"contentFormat": []string{"markdown", "plaintext"},
			},
			"definition":        map[string]any{},
			"declaration":       map[string]any{},
			"typeDefinition":    map[string]any{},
			"references":        map[string]any{},
			"documentSymbol":    map[string]any{},
			"completion":        map[string]any{},
			"signatureHelp":     map[string]any{},
			"formatting":        map[string]any{},
			"rename":            map[string]any{"prepareSupport": true},
			"documentHighlight": map[string]any{},
			"inlayHint":         map[string]any{},
			// zls gates push diagnostics on this capability being present.
			"publishDiagnostics": map[string]any{},
			"codeAction": map[string]any{
				"codeActionLiteralSupport": map[string]any{
					"codeActionKind": map[string]any{
						"valueSet": []string{
							"quickfix",
							"refactor",
							"source",
							"source.organizeImports",
							"source.fixAll",
						},
					},
				},
			},
			"foldingRange": map[string]any{
				"lineFoldingOnly": false,
			},
			"selectionRange": map[string]any{},
			"semanticTokens": map[string]any{
				"requests": map[string]any{
					"full":  true,
					"range": true,
				},
				"tokenTypes": []string{
					"namespace", "type", "class",
					"enum", "interface", "struct",
					"typeParameter", "parameter",
					"variable", "property",
					"enumMember", "event",
					"function", "method", "macro",
					"keyword", "modifier",
					"comment", "string", "number",
					"regexp", "operator",
					"decorator", "label",
				},
				"tokenModifiers": []string{
					"declaration", "definition",
					"readonly", "static",
					"deprecated", "abstract",
					"async", "modification",
					"documentation",
					"defaultLibrary",
				},
				"formats": []string{"relative"},
			},
		},
		"workspace": map[string]any{
			"symbol": map[string]any{
				"symbolKind": map[string]any{
					"valueSet": []int{
						int(semanticapi.SymbolKindFile),
						int(semanticapi.SymbolKindModule),
						int(semanticapi.SymbolKindNamespace),
						int(semanticapi.SymbolKindPackage),
						int(semanticapi.SymbolKindClass),
						int(semanticapi.SymbolKindMethod),
						int(semanticapi.SymbolKindProperty),
						int(semanticapi.SymbolKindField),
						int(semanticapi.SymbolKindConstructor),
						int(semanticapi.SymbolKindEnum),
						int(semanticapi.SymbolKindInterface),
						int(semanticapi.SymbolKindFunction),
						int(semanticapi.SymbolKindVariable),
						int(semanticapi.SymbolKindConstant),
						int(semanticapi.SymbolKindString),
						int(semanticapi.SymbolKindNumber),
						int(semanticapi.SymbolKindBoolean),
						int(semanticapi.SymbolKindArray),
						int(semanticapi.SymbolKindObject),
						int(semanticapi.SymbolKindKey),
						int(semanticapi.SymbolKindNull),
						int(semanticapi.SymbolKindEnumMember),
						int(semanticapi.SymbolKindStruct),
						int(semanticapi.SymbolKindEvent),
						int(semanticapi.SymbolKindOperator),
						int(semanticapi.SymbolKindTypeParameter),
					},
				},
			},
			"workspaceEdit": map[string]any{
				"documentChanges": true,
			},
			"configuration": true,
		},
		"window": map[string]any{
			"workDoneProgress": true,
			"showDocument": map[string]any{
				"support": true,
			},
		},
	}

	capabilitiesData, err := json.Marshal(capabilities)
	if err != nil {
		return semanticapi.InitializeParams{}, fmt.Errorf("marshal capabilities: %w", err)
	}

	return semanticapi.InitializeParams{
		RootURI:      rootURI,
		Capabilities: json.RawMessage(capabilitiesData),
		WorkspaceFolders: []semanticapi.WorkspaceFolder{
			{URI: rootURI, Name: rootName},
		},
		InitializeOptions: json.RawMessage(initOptionsData),
		Trace:             semanticapi.TraceValueOff,
	}, nil
}
