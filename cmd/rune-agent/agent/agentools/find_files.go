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

package agentools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/cmd/rune-agent/agent"
	"unstable.build/rune/internal/workspace/walkdir"
)

const maxFindResults = 500

type findFilesTool struct {
	fs      workspaceapi.FileSystem
	cwd     workspaceapi.URI
	tracker *FileTracker
	filter  walkdir.Filter
}

type findFilesArgs struct {
	Pattern   string `json:"pattern"`
	Path      string `json:"path"`
	Recursive *bool  `json:"recursive"`
}

func newFindFiles(wfs workspaceapi.FileSystem, cwd workspaceapi.URI, tracker *FileTracker, filter walkdir.Filter) agent.Tool {
	return &findFilesTool{fs: wfs, cwd: cwd, tracker: tracker, filter: filter}
}

func (t *findFilesTool) NeedsDeterministicOrder() bool { return false }

func (t *findFilesTool) Definition() llmapi.Tool {
	return llmapi.Tool{
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{
			Name: "find_files",
			Description: `Find files by name or path using a regex pattern. Searches the
directory starting from path (defaults to workspace root), skipping
.git, node_modules, and vendor directories as well as gitignored files.
Set recursive to false to inspect only files whose immediate parent is
path. Set recursive to true to also inspect files in every descendant
subdirectory under path, at any depth.

Returns matching file paths relative to the workspace root, one per
line, capped at 500 results. The pattern is matched against the full
relative path — use "\.go$" to find all Go files, "Makefile" for
exact names, "test" for any path containing test, or
"cmd/.*main" to match within specific directories. The pattern uses
Go regex (RE2) syntax.`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"pattern": map[string]any{
						"type":        "string",
						"description": "Go regex (RE2) pattern matched against the full relative path.",
					},
					"path": map[string]any{
						"type":        []string{"string", "null"},
						"description": "Optional directory to search in (relative to workspace root or absolute). Defaults to workspace root.",
					},
					"recursive": map[string]any{
						"type":        "boolean",
						"description": "Whether to search subdirectories under path. When false, matches can include only files directly inside path; files in child directories are excluded. When true, matches can also include files in every descendant subdirectory under path, at any depth.",
					},
				},
				"required":             []string{"pattern", "path", "recursive"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *findFilesTool) Summary(arguments string) string {
	var args findFilesArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}
	s := `"` + args.Pattern + `"`
	if args.Path != "" {
		s += " in " + summaryPath(t.cwd, args.Path)
	}
	return s
}

func (t *findFilesTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	var args findFilesArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: invalid arguments: %v", err), IsError: true}
	}

	re, err := regexp.Compile(args.Pattern)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: invalid regex: %v", err), IsError: true}
	}

	root := t.cwd.Path()
	if args.Path != "" {
		root = resolvePath(t.cwd, args.Path)
	}
	walkCtx := walkdir.WithContextFilter(boundedWalkdirContext(ctx), t.filter)
	wsRoot := t.cwd.Path()

	recursive := true
	if args.Recursive != nil {
		recursive = *args.Recursive
	}
	paths, err := t.listFiles(walkCtx, root, recursive)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: listing files: %v", err), IsError: true}
	}

	filtered := iterator.Filter(paths, func(path string) bool {
		for _, part := range strings.Split(filepath.Dir(path), string(filepath.Separator)) {
			if skipDirs[part] {
				return false
			}
		}
		return re.MatchString(path)
	})
	defer func() { _ = filtered.Close() }()

	var results []string
	truncated := false
	for {
		path, ok := filtered.Next(ctx)
		if !ok {
			break
		}
		results = append(results, path)
		if len(results) >= maxFindResults {
			truncated = true
			break
		}
	}

	var iterErr error
	if !truncated {
		iterErr = filtered.Err()
	}
	warning, fatal := walkIterErr(ctx, iterErr, len(results))
	if fatal {
		return agent.ToolResult{Content: warning, IsError: true}
	}

	if len(results) == 0 {
		return agent.ToolResult{Content: "no files found"}
	}

	// Track discovered paths so file-reading tools can consume them.
	discoveredPaths := make([]string, len(results))
	for i, relPath := range results {
		discoveredPaths[i] = filepath.Join(wsRoot, relPath)
	}
	t.tracker.TrackDiscovery(ctx, discoveredPaths)

	output := strings.Join(results, "\n")
	if truncated {
		output += fmt.Sprintf("\n\n(results truncated at %d files)", maxFindResults)
	}
	if warning != "" {
		output += "\n\n" + warning
	}
	return agent.ToolResult{Content: output}
}

func (t *findFilesTool) listFiles(
	ctx context.Context, root string, recursive bool,
) (iterator.Iterator[string], error) {
	if recursive {
		return listToolFiles(ctx, t.fs, t.cwd, root, t.filter)
	}

	info, err := t.fs.Stat(root)
	if err != nil {
		return nil, err
	}
	if info.Mode().IsRegular() {
		return listToolFiles(ctx, t.fs, t.cwd, root, t.filter)
	}
	if !info.IsDir() {
		return iterator.Empty[string](), nil
	}
	entries, err := t.fs.ReadDir(root)
	if err != nil {
		return nil, err
	}
	rootURI, err := t.fs.URI(root)
	if err != nil {
		return nil, fmt.Errorf("URI: %v", err)
	}
	relRoot := workspaceapi.RelPath(t.cwd, rootURI)
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !entry.Type().IsRegular() {
			continue
		}
		path := filepath.Join(relRoot, entry.Name())
		if t.filter != nil && t.filter.MatchRelPath(path, false) {
			continue
		}
		paths = append(paths, path)
	}
	return iterator.FromSlice(paths), nil
}
