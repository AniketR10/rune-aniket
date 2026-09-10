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
	"log/slog"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/cmd/rune-agent/agent"
	"unstable.build/rune/cmd/rune-agent/agent/agentools/applypatch"
)

type applyPatchTool struct {
	fs      workspaceapi.FileSystem
	cwd     workspaceapi.URI
	tracker *FileTracker
	lsp     semanticapi.LSP
}

type applyPatchArgs struct {
	Patch string `json:"patch"`
}

func newApplyPatch(
	fs workspaceapi.FileSystem,
	cwd workspaceapi.URI,
	tracker *FileTracker,
	lsp semanticapi.LSP,
) agent.Tool {
	return &applyPatchTool{fs: fs, cwd: cwd, tracker: tracker, lsp: lsp}
}

func (t *applyPatchTool) Definition() llmapi.Tool {
	return llmapi.Tool{
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{
			Name: "apply_patch",
			Description: `Apply a patch to create, update, or delete files. The patch uses a unified diff format.

Format:
` + "```" + `
*** Begin Patch
*** Add File: <path>
+<line1>
+<line2>
*** Update File: <path>
*** Move to: <new_path>  (optional: renames the file)
@@ <optional context hint>
 <context line>
-<removed line>
+<added line>
 <context line>
*** Delete File: <path>
*** End Patch
` + "```" + `

Rules:
- The patch MUST start with '*** Begin Patch' and end with '*** End Patch'
- Every line in an Add block must start with '+'
- In Update hunks: ' ' = context (unchanged), '-' = remove, '+' = add
- Context lines must match the existing file (fuzzy whitespace matching is supported)
- Multiple hunks per file are applied in order; ensure hunks don't overlap
- Always read a file before updating it`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"patch": map[string]any{
						"type":        "string",
						"description": "The patch content in the format described above.",
					},
				},
				"required":             []string{"patch"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *applyPatchTool) Summary(arguments string) string {
	var args applyPatchArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}

	var paths []string
	for line := range strings.SplitSeq(args.Patch, "\n") {
		trimmed := strings.TrimSpace(line)
		for _, prefix := range []string{"*** Add File:", "*** Update File:", "*** Delete File:"} {
			if after, ok := strings.CutPrefix(trimmed, prefix); ok {
				paths = append(paths, strings.TrimSpace(after))
			}
		}
	}
	return strings.Join(paths, ", ")
}

func (t *applyPatchTool) NeedsDeterministicOrder() bool { return true }

func (t *applyPatchTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	var args applyPatchArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: invalid arguments: %v", err), IsError: true}
	}

	patch, err := applypatch.Parse(args.Patch)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: parse patch: %v", err), IsError: true}
	}

	// Pre-verify: ensure files to be modified haven't changed
	// since the agent last read them.
	for _, op := range patch.Ops {
		if op.Type == applypatch.OpAdd {
			continue
		}
		path := resolvePath(t.cwd, op.Path)
		data, err := readFile(t.fs, path)
		if err != nil {
			continue // Apply will report this error.
		}
		if err := t.tracker.Verify(path, data); err != nil {
			return agent.ToolResult{Content: fmt.Sprintf("error: %v", err), IsError: true}
		}
	}

	result := applypatch.Apply(t.fs, t.cwd, patch)

	// Collect stale read IDs and consumed discovery IDs, and clear
	// tracked hashes for affected files so the agent re-reads before
	// the next modification.
	var staleIDs []string
	for _, op := range patch.Ops {
		path := resolvePath(t.cwd, op.Path)
		staleIDs = append(staleIDs, t.tracker.StaleReads(path)...)
		staleIDs = append(staleIDs, t.tracker.ConsumeDiscoveries(path)...)
		t.tracker.Forget(path)
		if op.MoveTo != "" {
			movePath := resolvePath(t.cwd, op.MoveTo)
			staleIDs = append(staleIDs, t.tracker.StaleReads(movePath)...)
			staleIDs = append(staleIDs, t.tracker.ConsumeDiscoveries(movePath)...)
			t.tracker.Forget(movePath)
		}
	}

	// Collect touched file paths (non-delete operations) for
	// downstream auto-diagnostics.
	var touchedFiles []string
	for _, op := range patch.Ops {
		if op.Type == applypatch.OpDelete {
			continue
		}
		p := op.Path
		if op.MoveTo != "" {
			p = op.MoveTo
		}
		touchedFiles = append(touchedFiles, resolvePath(t.cwd, p))
	}

	if len(result.Errors) > 0 {
		return agent.ToolResult{
			Content: fmt.Sprintf("applied %d/%d operations; errors:\n%s",
				result.Applied, result.Total, strings.Join(result.Errors, "\n")),
			IsError:           true,
			DropToolResultIDs: staleIDs,
		}
	}

	// Notify the LSP subsystem that files changed on disk so the
	// callback marks them as pending before auto-diagnostics run.
	t.notifyFileChanges(ctx, patch)

	return agent.ToolResult{
		Content:           fmt.Sprintf("applied %d/%d operations successfully", result.Applied, result.Total),
		DropToolResultIDs: staleIDs,
		TouchedFiles:      touchedFiles,
	}
}

// notifyFileChanges sends workspace/didChangeWatchedFiles for each
// operation in the patch. This is a synchronous gRPC call that ensures
// the Manager records pending state before any subsequent Diagnostic
// call.
func (t *applyPatchTool) notifyFileChanges(ctx context.Context, patch applypatch.Patch) {
	var events []semanticapi.FileEvent
	for _, op := range patch.Ops {
		uri, err := fileURI(t.fs, t.cwd, op.Path)
		if err != nil {
			continue
		}
		switch op.Type {
		case applypatch.OpAdd:
			events = append(events, semanticapi.FileEvent{
				URI:  uri.String(),
				Type: semanticapi.FileChangeTypeCreated,
			})
		case applypatch.OpUpdate:
			events = append(events, semanticapi.FileEvent{
				URI:  uri.String(),
				Type: semanticapi.FileChangeTypeChanged,
			})
			if op.MoveTo != "" {
				// Rename = delete old + create new.
				events = append(events, semanticapi.FileEvent{
					URI:  uri.String(),
					Type: semanticapi.FileChangeTypeDeleted,
				})
				newURI, err := fileURI(t.fs, t.cwd, op.MoveTo)
				if err == nil {
					events = append(events, semanticapi.FileEvent{
						URI:  newURI.String(),
						Type: semanticapi.FileChangeTypeCreated,
					})
				}
			}
		case applypatch.OpDelete:
			events = append(events, semanticapi.FileEvent{
				URI:  uri.String(),
				Type: semanticapi.FileChangeTypeDeleted,
			})
		}
	}
	if len(events) == 0 {
		return
	}
	if err := t.lsp.DidChangeWatchedFiles(ctx, semanticapi.DidChangeWatchedFilesParams{
		Changes: events,
	}); err != nil {
		slog.Warn("apply_patch: notify file changes", "error", err)
	}
}
