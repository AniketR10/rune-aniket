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
	"log/slog"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/agentools/applypatch"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
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
