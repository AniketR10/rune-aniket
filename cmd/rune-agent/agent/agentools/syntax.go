// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package agentools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/cmd/rune-agent/agent"
)

// SyntaxTools returns all syntax (tree-sitter) backed agent tools.
// The tracker should be the same instance returned by DefaultTools so
// that stale reads are shared across all file-aware tools.
func SyntaxTools(
	parser syntaxapi.Parser,
	fs workspaceapi.FileSystem,
	cwd workspaceapi.URI,
	tracker *FileTracker,
) []agent.Tool {
	return []agent.Tool{
		&listSymbolsTool{parser: parser, cwd: cwd, tracker: tracker},
		&listFileSymbolsTool{parser: parser, fs: fs, cwd: cwd, tracker: tracker},
		&queryASTTool{parser: parser, cwd: cwd, tracker: tracker},
		&queryFileASTTool{parser: parser, fs: fs, cwd: cwd, tracker: tracker},
	}
}

// parseNodeTypes parses a pipe-separated string of node type names into a bitmask.
// Valid names: scope, namespace, reference, func, method, type, var.
func parseNodeTypes(s string) (syntaxapi.NodeCaptureName, error) {
	var result syntaxapi.NodeCaptureName
	for p := range strings.SplitSeq(s, "|") {
		p = strings.TrimSpace(p)
		switch strings.ToLower(p) {
		case "scope":
			result |= syntaxapi.NodeCaptureScope
		case "namespace":
			result |= syntaxapi.NodeCaptureDefinitionNamespace
		case "reference":
			result |= syntaxapi.NodeCaptureReference
		case "func":
			result |= syntaxapi.NodeCaptureDefinitionFunc
		case "var":
			result |= syntaxapi.NodeCaptureDefinitionVar
		case "method":
			result |= syntaxapi.NodeCaptureDefinitionMethod
		case "type":
			result |= syntaxapi.NodeCaptureDefinitionType
		default:
			return 0, fmt.Errorf("unknown node type %q (valid: scope, namespace, reference, func, method, type, var)", p)
		}
	}
	if result == 0 {
		return 0, fmt.Errorf("no valid node types specified")
	}
	return result, nil
}

// collectResults drains up to limit results from an iterator.
func collectResults(
	ctx context.Context, it iterator.Iterator[syntaxapi.Result], limit int,
) (results []syntaxapi.Result, truncated bool) {
	defer it.Close() //nolint:errcheck
	for {
		r, ok := it.Next(ctx)
		if !ok {
			break
		}
		results = append(results, r)
		if len(results) >= limit {
			truncated = true
			break
		}
	}
	return results, truncated
}

// formatSyntaxResult formats a single syntax result.
func formatSyntaxResult(r syntaxapi.Result, cwd workspaceapi.URI) string {
	rel := workspaceapi.RelPath(cwd, r.File)
	// From.Y is 0-based row, From.X is 0-based col
	line := r.From.Y + 1
	return fmt.Sprintf("%s:%d:%s", rel, line, r.Text)
}

// formatSyntaxResults formats a slice of syntax results.
func formatSyntaxResults(results []syntaxapi.Result, truncated bool, cwd workspaceapi.URI) string {
	if len(results) == 0 {
		return "no results found"
	}
	var sb strings.Builder
	for _, r := range results {
		sb.WriteString(formatSyntaxResult(r, cwd))
		sb.WriteByte('\n')
	}
	if truncated {
		fmt.Fprintf(&sb, "\n(results truncated at %d matches)", maxSearchResults)
	}
	return sb.String()
}

// pathsFromSyntaxResults extracts unique absolute file paths from syntax results.
func pathsFromSyntaxResults(results []syntaxapi.Result) []string {
	seen := make(map[string]struct{})
	var paths []string
	for _, r := range results {
		p := r.File.Path()
		if _, ok := seen[p]; !ok {
			seen[p] = struct{}{}
			paths = append(paths, p)
		}
	}
	return paths
}

// --- list_symbols ---

type listSymbolsTool struct {
	parser  syntaxapi.Parser
	cwd     workspaceapi.URI
	tracker *FileTracker
}

type nodeTypesArgs struct {
	NodeTypes string `json:"node_types"`
}

func (t *listSymbolsTool) NeedsDeterministicOrder() bool { return false }

func (t *listSymbolsTool) Definition() llmapi.Tool {
	return llmapi.Tool{
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{
			Name: "list_symbols",
			Description: `Find all functions, methods, types, or variables across the workspace by
category using tree-sitter. Returns matches formatted as
"path:line:text", capped at 200 results.

Specify node_types as pipe-separated values: func, method, type, var,
scope, namespace, reference. For example, "func|method" finds all
function and method definitions. Use when you need a broad category
search rather than a name-based search (use search_symbols for that).`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"node_types": map[string]any{
						"type":        "string",
						"description": "Pipe-separated node types to search for (e.g. \"func\", \"func|type|var\").",
					},
				},
				"required":             []string{"node_types"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *listSymbolsTool) Summary(arguments string) string {
	var args nodeTypesArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}
	return args.NodeTypes
}

func (t *listSymbolsTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	var args nodeTypesArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: invalid arguments: %v", err), IsError: true}
	}
	nodeTypes, err := parseNodeTypes(args.NodeTypes)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: %v", err), IsError: true}
	}
	it, err := t.parser.SearchNode(nodeTypes)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: search node: %v", err), IsError: true}
	}
	results, truncated := collectResults(ctx, it, maxSearchResults)
	t.tracker.TrackDiscovery(ctx, pathsFromSyntaxResults(results))
	return agent.ToolResult{Content: formatSyntaxResults(results, truncated, t.cwd)}
}

// --- list_file_symbols ---

type listFileSymbolsTool struct {
	parser  syntaxapi.Parser
	fs      workspaceapi.FileSystem
	cwd     workspaceapi.URI
	tracker *FileTracker
}

type fileNodeTypesArgs struct {
	filePathArgs
	NodeTypes string `json:"node_types"`
}

func (t *listFileSymbolsTool) NeedsDeterministicOrder() bool { return false }

func (t *listFileSymbolsTool) Definition() llmapi.Tool {
	return llmapi.Tool{
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{
			Name: "list_file_symbols",
			Description: `Find functions, methods, types, or variables in a single file by
category using tree-sitter. Same as list_symbols but scoped to one file.

Returns matches formatted as "path:line:text", capped at 200
results. Specify node_types as pipe-separated values: func, method,
type, var, scope, namespace, reference.`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "The file path to search in (relative to workspace root or absolute).",
					},
					"node_types": map[string]any{
						"type":        "string",
						"description": "Pipe-separated node types to search for (e.g. \"func\", \"func|type|var\").",
					},
				},
				"required":             []string{"path", "node_types"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *listFileSymbolsTool) Summary(arguments string) string {
	var args fileNodeTypesArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}
	fp, _ := args.filePath()
	return summaryPath(t.cwd, fp) + " " + args.NodeTypes
}

func (t *listFileSymbolsTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	var args fileNodeTypesArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: invalid arguments: %v", err), IsError: true}
	}
	fp, err := args.filePath()
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: %v", err), IsError: true}
	}
	uri, err := fileURI(t.fs, t.cwd, fp)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: resolve path: %v", err), IsError: true}
	}
	nodeTypes, err := parseNodeTypes(args.NodeTypes)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: %v", err), IsError: true}
	}
	it, err := t.parser.QueryNode(uri, nodeTypes)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: query node: %v", err), IsError: true}
	}
	results, truncated := collectResults(ctx, it, maxSearchResults)
	absPath := resolvePath(t.cwd, fp)
	staleIDs := t.tracker.TrackRead(ctx, "list_file_symbols", absPath, "")
	staleIDs = append(staleIDs, t.tracker.ConsumeDiscoveries(absPath)...)
	return agent.ToolResult{
		Content:           formatSyntaxResults(results, truncated, t.cwd),
		DropToolResultIDs: staleIDs,
	}
}

// --- query_ast ---

type queryASTTool struct {
	parser  syntaxapi.Parser
	cwd     workspaceapi.URI
	tracker *FileTracker
}

type queryASTArgs struct {
	Query    string   `json:"query"`
	Captures []string `json:"captures"`
}

func (t *queryASTTool) NeedsDeterministicOrder() bool { return false }

func (t *queryASTTool) Definition() llmapi.Tool {
	return llmapi.Tool{
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{
			Name: "query_ast",
			Description: `Search for structural code patterns across the workspace using a
tree-sitter S-expression query. Returns matches formatted as
"path:line:text", capped at 200 results.

Use for patterns that regex cannot express — for example:
- All function calls with two arguments:
  (call_expression arguments: (argument_list (_) (_)))
- All assignments to a specific variable:
  (assignment_statement left: (identifier) @name)

Use captures to filter which parts of the match to return. More precise
than regex-based search_content.`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{
						"type":        "string",
						"description": "Tree-sitter S-expression query pattern.",
					},
					"captures": map[string]any{
						"type":        []string{"array", "null"},
						"items":       map[string]any{"type": "string"},
						"description": "Optional capture names to filter results (e.g. [\"definition.function\"]).",
					},
				},
				"required":             []string{"query", "captures"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *queryASTTool) Summary(arguments string) string {
	var args queryASTArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}
	q := args.Query
	if len(q) > 60 {
		q = q[:57] + "..."
	}
	return q
}

func (t *queryASTTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	var args queryASTArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: invalid arguments: %v", err), IsError: true}
	}
	it, err := t.parser.Search(args.Query, args.Captures)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: search: %v", err), IsError: true}
	}
	results, truncated := collectResults(ctx, it, maxSearchResults)
	t.tracker.TrackDiscovery(ctx, pathsFromSyntaxResults(results))
	return agent.ToolResult{Content: formatSyntaxResults(results, truncated, t.cwd)}
}

// --- query_file_ast ---

type queryFileASTTool struct {
	parser  syntaxapi.Parser
	fs      workspaceapi.FileSystem
	cwd     workspaceapi.URI
	tracker *FileTracker
}

type queryFileASTArgs struct {
	filePathArgs
	Query    string   `json:"query"`
	Captures []string `json:"captures"`
}

func (t *queryFileASTTool) NeedsDeterministicOrder() bool { return false }

func (t *queryFileASTTool) Definition() llmapi.Tool {
	return llmapi.Tool{
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{
			Name: "query_file_ast",
			Description: `Search for structural code patterns in a single file using a tree-sitter
S-expression query. Same as query_ast but scoped to one file.

Returns matches formatted as "path:line:text", capped at 200
results. Use captures to filter which parts of the match to return.`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "The file path to query (relative to workspace root or absolute).",
					},
					"query": map[string]any{
						"type":        "string",
						"description": "Tree-sitter S-expression query pattern.",
					},
					"captures": map[string]any{
						"type":        []string{"array", "null"},
						"items":       map[string]any{"type": "string"},
						"description": "Optional capture names to filter results (e.g. [\"definition.function\"]).",
					},
				},
				"required":             []string{"path", "query", "captures"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *queryFileASTTool) Summary(arguments string) string {
	var args queryFileASTArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}
	fp, _ := args.filePath()
	return summaryPath(t.cwd, fp)
}

func (t *queryFileASTTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	var args queryFileASTArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: invalid arguments: %v", err), IsError: true}
	}
	fp, err := args.filePath()
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: %v", err), IsError: true}
	}
	uri, err := fileURI(t.fs, t.cwd, fp)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: resolve path: %v", err), IsError: true}
	}
	it, err := t.parser.Query(uri, args.Query, args.Captures)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: query: %v", err), IsError: true}
	}
	results, truncated := collectResults(ctx, it, maxSearchResults)
	absPath := resolvePath(t.cwd, fp)
	staleIDs := t.tracker.TrackRead(ctx, "query_file_ast", absPath, "")
	staleIDs = append(staleIDs, t.tracker.ConsumeDiscoveries(absPath)...)
	return agent.ToolResult{
		Content:           formatSyntaxResults(results, truncated, t.cwd),
		DropToolResultIDs: staleIDs,
	}
}
