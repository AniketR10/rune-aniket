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
	"path/filepath"
	"sort"
	"strings"

	"github.com/unstablebuild/blue/walkdir"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/llm"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

const (
	defaultListDirDepth  = 2
	defaultListDirOffset = 1
	defaultListDirLimit  = 25
)

type listDirTool struct {
	fs  workspaceapi.FileSystem
	cwd workspaceapi.URI
}

type listDirArgs struct {
	DirPath string `json:"dir_path"`
	Depth   int    `json:"depth"`
	Offset  int    `json:"offset"`
	Limit   int    `json:"limit"`
}

// NewListDir creates a tool that lists directory contents relative to cwd.
func NewListDir(wfs workspaceapi.FileSystem, cwd workspaceapi.URI) agent.Tool {
	return &listDirTool{fs: wfs, cwd: cwd}
}

func (t *listDirTool) Definition() llm.Tool {
	return llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name:        "list_dir",
			Description: "Lists entries in a local directory with 1-indexed entry numbers and simple type labels.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"depth": map[string]any{
						"type":        "number",
						"description": "The maximum directory depth to traverse. Must be 1 or greater.",
					},
					"dir_path": map[string]any{
						"type":        "string",
						"description": "Absolute path to the directory to list.",
					},
					"limit": map[string]any{
						"type":        "number",
						"description": "The maximum number of entries to return.",
					},
					"offset": map[string]any{
						"type":        "number",
						"description": "The entry number to start listing from. Must be 1 or greater.",
					},
				},
				"required":             []string{"dir_path"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *listDirTool) Summary(arguments string) string {
	var args listDirArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}
	return summaryPath(t.cwd, args.DirPath)
}

func (t *listDirTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	var args listDirArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: invalid arguments: %v", err), IsError: true}
	}

	depth := args.Depth
	if depth <= 0 {
		depth = defaultListDirDepth
	}
	offset := args.Offset
	if offset <= 0 {
		offset = defaultListDirOffset
	}
	limit := args.Limit
	if limit <= 0 {
		limit = defaultListDirLimit
	}

	root := resolvePath(t.cwd, args.DirPath)

	// Validate that root exists and is a directory.
	info, err := t.fs.Stat(root)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: %v", err), IsError: true}
	}
	if !info.IsDir() {
		return agent.ToolResult{Content: fmt.Sprintf("error: %s is not a directory", root), IsError: true}
	}

	// List all files under root.
	fileIter, err := walkdir.ListFiles(ctx, t.fs, root)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: listing files: %v", err), IsError: true}
	}
	defer func() { _ = fileIter.Close() }()

	// Build entries from file paths, deriving directory entries from
	// intermediate path components.
	type entry struct {
		relPath string
		isDir   bool
	}
	seen := make(map[string]bool)
	var entries []entry

	for {
		path, ok := fileIter.Next(ctx)
		if !ok {
			break
		}

		d := pathDepth(path)
		parts := strings.Split(path, string(filepath.Separator))

		// Add ancestor directories up to the depth limit.
		for i := 1; i < d && i <= depth; i++ {
			dirPath := strings.Join(parts[:i], string(filepath.Separator))
			if !seen[dirPath] {
				seen[dirPath] = true
				entries = append(entries, entry{relPath: dirPath, isDir: true})
			}
		}

		// Add the file itself if within depth.
		if d <= depth {
			entries = append(entries, entry{relPath: path, isDir: false})
		}
	}

	// Sort alphabetically by relative path.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].relPath < entries[j].relPath
	})

	// Apply 1-indexed offset and limit.
	start := offset - 1
	if start > len(entries) {
		start = len(entries)
	}
	page := entries[start:]
	hasMore := len(page) > limit
	if hasMore {
		page = page[:limit]
	}

	// Format output.
	var sb strings.Builder
	sb.WriteString("Absolute path: ")
	sb.WriteString(root)
	for _, e := range page {
		sb.WriteByte('\n')
		d := pathDepth(e.relPath)
		for i := 1; i < d; i++ {
			sb.WriteString("  ")
		}
		sb.WriteString(filepath.Base(e.relPath))
		if e.isDir {
			sb.WriteByte('/')
		}
	}
	if hasMore {
		fmt.Fprintf(&sb, "\nMore than %d entries found", limit)
	}

	return agent.ToolResult{Content: sb.String()}
}

// pathDepth returns the number of components in a relative path.
func pathDepth(path string) int {
	if path == "" || path == "." {
		return 0
	}
	return strings.Count(path, string(filepath.Separator)) + 1
}
