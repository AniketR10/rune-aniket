// Copyright (C) 2017-2026 Unstable Build, LLC
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

package agentools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/cmd/rune-agent/agent"
	"unstable.build/rune/internal/workspace/walkdir"
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

func (t *listDirTool) NeedsDeterministicOrder() bool { return false }

func (t *listDirTool) Definition() llmapi.Tool {
	return llmapi.Tool{
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{
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
	walkCtx := boundedWalkdirContext(ctx)

	// List all files under root.
	fileIter, err := walkdir.ListFiles(walkCtx, t.fs, root)
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

		relPath := path
		if filepath.IsAbs(path) {
			var err error
			relPath, err = filepath.Rel(root, path)
			if err != nil {
				return agent.ToolResult{Content: fmt.Sprintf("error: relativizing path %s: %v", path, err), IsError: true}
			}
		}
		if relPath == "." {
			continue
		}

		d := pathDepth(relPath)
		parts := strings.Split(relPath, string(filepath.Separator))

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
			entries = append(entries, entry{relPath: relPath, isDir: false})
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
