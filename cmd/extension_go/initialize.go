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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
)

// goplsDebugOptions controls gopls debug instrumentation. All fields are
// optional; the zero value keeps gopls's normal stderr logging.
type goplsDebugOptions struct {
	// RPCTrace adds `-rpc.trace` to the gopls command, causing gopls to
	// log every JSON-RPC message it sends/receives.
	RPCTrace bool
	// LogFile adds `-logfile=PATH` to the gopls command. The literal
	// "auto" makes gopls write to a per-pid file under $TMPDIR.
	LogFile string
	// DebugAddr adds `-debug=ADDR` to the gopls command, enabling the
	// gopls HTTP debug endpoint (pprof, /rpc, /cache, /session/*, …) at
	// http://ADDR/. Example: "localhost:6060".
	DebugAddr string
	LogLevel  string
	// Trace sets the InitializeParams.trace value, which controls
	// $/logTrace messages from the server. Valid values are "off",
	// "messages", and "verbose".
	Trace semanticapi.TraceValue
}

func goplsInitializeParams(
	rootURI string, dbg goplsDebugOptions, binPath string,
) (semanticapi.InitializeParams, error) {
	command := goplsCommand(dbg, binPath)
	initOptions := map[string]any{
		"langID":         "go",
		"command":        command,
		"semanticTokens": true,
		"codelenses": map[string]any{
			"gc_details":         true,
			"generate":           true,
			"regenerate_cgo":     true,
			"run_govulncheck":    true,
			"test":               true,
			"tidy":               true,
			"upgrade_dependency": true,
			"vendor":             true,
		},
		"hints": map[string]any{
			"assignVariableTypes":    true,
			"compositeLiteralFields": true,
			"compositeLiteralTypes":  true,
			"constantValues":         true,
			"functionTypeParameters": true,
			"parameterNames":         true,
			"rangeVariableTypes":     true,
		},
	}

	initOptionsData, err := json.Marshal(initOptions)
	if err != nil {
		return semanticapi.InitializeParams{}, fmt.Errorf("marshal initialize options: %w", err)
	}

	capabilities := map[string]any{
		"textDocument": map[string]any{
			"implementation": map[string]any{
				"linkSupport": true,
			},
			"completion":     map[string]any{},
			"hover":          map[string]any{},
			"signatureHelp":  map[string]any{},
			"definition":     map[string]any{},
			"declaration":    map[string]any{},
			"typeDefinition": map[string]any{},
			"references":     map[string]any{},
			"documentSymbol": map[string]any{},
			"formatting":     map[string]any{},
			"rename": map[string]any{
				"prepareSupport": true,
			},
			"codeAction": map[string]any{
				"codeActionLiteralSupport": map[string]any{
					"codeActionKind": map[string]any{
						"valueSet": []string{
							"quickfix",
							"refactor",
							"refactor.extract",
							"refactor.inline",
							"refactor.rewrite",
							"source",
							"source.organizeImports",
							"source.fixAll",
						},
					},
				},
			},
			"codeLens": map[string]any{},
			"foldingRange": map[string]any{
				"lineFoldingOnly": false,
			},
			"selectionRange":    map[string]any{},
			"documentHighlight": map[string]any{},
			"callHierarchy":     map[string]any{},
			"inlayHint":         map[string]any{},
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
			"diagnostics": map[string]any{},
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
		RootURI:           rootURI,
		Capabilities:      json.RawMessage(capabilitiesData),
		InitializeOptions: json.RawMessage(initOptionsData),
		Trace:             dbg.Trace,
	}, nil
}

// goplsCommand builds the gopls invocation. Debug flags are top-level
// flags on the gopls binary itself and must appear *before* the `serve`
// subcommand; placing them after `serve` makes gopls exit with status 2.
//
// binPath overrides the binary location. When empty, the bare name
// "gopls" is used and the executor must resolve it through $PATH.
func goplsCommand(dbg goplsDebugOptions, binPath string) string {
	bin := binPath
	if bin == "" {
		bin = "gopls"
	}
	parts := []string{bin}
	if dbg.RPCTrace {
		parts = append(parts, "-rpc.trace")
	}
	if dbg.LogFile != "" {
		parts = append(parts, "-logfile="+dbg.LogFile)
	}
	if dbg.DebugAddr != "" {
		parts = append(parts, "-debug="+dbg.DebugAddr)
	}
	switch dbg.LogLevel {
	case "debug":
		parts = append(parts, "-v")
	case "trace":
		parts = append(parts, "-vv")
	}
	parts = append(parts, "serve")
	return strings.Join(parts, " ")
}
