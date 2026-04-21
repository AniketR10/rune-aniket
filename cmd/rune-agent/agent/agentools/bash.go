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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/llm"
)

const (
	maxCommandOutput = 100 * 1024 // 100KB
)

type bashTool struct {
	exec workspaceapi.Executor
	cwd  workspaceapi.URI
}

type bashArgs struct {
	Command     string `json:"command"`
	Description string `json:"description"`
	WorkingDir  string `json:"working_dir"`
}

func newBash(exec workspaceapi.Executor, cwd workspaceapi.URI) agent.Tool {
	return &bashTool{exec: exec, cwd: cwd}
}

func (t *bashTool) Definition() llm.Tool {
	return llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name: "bash",
			Description: `Executes a bash command and returns its output.

Commands run in the workspace root by default — do NOT prepend cd to
the command. Use working_dir only if you need a different directory.
Each invocation starts a fresh shell; state does not persist between calls.

Output is truncated at 100 KB.

Do NOT use this tool for tasks that have a dedicated tool:
• grep/rg/ag → use search_content or find_files
• cat/head/tail → use read_file
• gofmt/goimports → use format_file
• find → use find_files
• Symbol search → use search_symbols or find_definition

Use bash only for running tests, build commands, git, or other tasks
with no dedicated tool.`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "The bash command to execute.",
					},
					"description": map[string]any{
						"type": "string",
						"description": "Clear, concise description of what " +
							"this command does (5-10 words for simple " +
							"commands, more for complex ones).",
					},
					"working_dir": map[string]any{
						"type": []string{"string", "null"},
						"description": "Working directory (relative to " +
							"workspace root or absolute). Defaults to " +
							"workspace root.",
					},
				},
				"required":             []string{"command", "description"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *bashTool) Summary(arguments string) string {
	var args bashArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}
	return args.Command
}

func (t *bashTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	var args bashArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: invalid arguments: %v", err), IsError: true}
	}

	workDir := t.cwd.Path()
	if args.WorkingDir != "" {
		workDir = resolvePath(t.cwd, args.WorkingDir)
	}

	var buf bytes.Buffer
	watcher := newProcessWatcher()

	cmd := workspaceapi.Cmd{
		Path:    "bash",
		Args:    []string{"-c", args.Command},
		Dir:     workDir,
		Env:     os.Environ(),
		Stdout:  &buf,
		Stderr:  &buf,
		Watcher: watcher,
	}

	_, err := t.exec.Start(ctx, cmd)
	if err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("error: %v", err),
			IsError: true,
		}
	}

	// Wait for process to finish or context cancellation.
	var procErr error
	select {
	case procErr = <-watcher.WatchProcess():
	case <-ctx.Done():
		procErr = ctx.Err()
	}

	output := buf.Bytes()

	// Truncate if too large
	if len(output) > maxCommandOutput {
		output = append(output[:maxCommandOutput],
			[]byte(fmt.Sprintf("\n\n(output truncated at %d bytes)", maxCommandOutput))...)
	}

	if procErr != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("%s\nerror: %v", string(output), procErr),
			IsError: true,
		}
	}

	content := string(output)
	if hint := bashToolHint(args.Command); hint != "" {
		content += "\n\nTIP: " + hint
	}
	return agent.ToolResult{Content: content}
}

type processWatcher struct {
	ch chan error
}

func newProcessWatcher() *processWatcher {
	return &processWatcher{ch: make(chan error, 1)}
}

func (w *processWatcher) WatchProcess() chan error {
	return w.ch
}
