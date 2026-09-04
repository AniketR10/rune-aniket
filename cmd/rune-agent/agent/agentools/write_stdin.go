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
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/rune/cmd/rune-agent/agent"
)

const (
	defaultWriteStdinYieldMs = 250
	minWriteStdinYieldMs     = 250
	maxWriteStdinYieldMs     = 30000
	emptyInputMinYieldMs     = 5000
)

type writeStdinTool struct {
	mgr *SessionManager
}

type writeStdinArgs struct {
	SessionID   int    `json:"session_id"`
	Chars       string `json:"chars"`
	YieldTimeMs *int   `json:"yield_time_ms"`
}

// NewWriteStdin creates a write_stdin tool that sends input to a
// running session's stdin and returns the current output.
func NewWriteStdin(mgr *SessionManager) agent.Tool {
	return &writeStdinTool{mgr: mgr}
}

func (t *writeStdinTool) Definition() llmapi.Tool {
	return llmapi.Tool{
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{
			Name:        "write_stdin",
			Description: "Writes characters to an existing unified exec session and returns recent output.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"session_id": map[string]any{
						"type":        "number",
						"description": "Identifier of the running unified exec session.",
					},
					"chars": map[string]any{
						"type":        "string",
						"description": "Bytes to write to stdin (may be empty to poll).",
					},
					"yield_time_ms": map[string]any{
						"type":        []string{"number", "null"},
						"description": "How long to wait (in milliseconds) for output before yielding.",
					},
				},
				"required":             []string{"session_id"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *writeStdinTool) Summary(arguments string) string {
	var args writeStdinArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}
	if args.Chars == "" {
		return fmt.Sprintf("poll session %d", args.SessionID)
	}
	chars := args.Chars
	if len(chars) > 40 {
		chars = chars[:40] + "..."
	}
	return fmt.Sprintf("session %d: %s", args.SessionID, chars)
}

func (t *writeStdinTool) NeedsDeterministicOrder() bool { return true }

func (t *writeStdinTool) Execute(_ context.Context, arguments string) agent.ToolResult {
	var args writeStdinArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: invalid arguments: %v", err), IsError: true}
	}

	sess, ok := t.mgr.Get(args.SessionID)
	if !ok {
		return agent.ToolResult{
			Content: fmt.Sprintf("error: session %d not found", args.SessionID),
			IsError: true,
		}
	}

	// Write input if non-empty.
	if args.Chars != "" {
		if err := sess.WriteStdin([]byte(args.Chars)); err != nil {
			return agent.ToolResult{
				Content: fmt.Sprintf("error: write stdin: %v", err),
				IsError: true,
			}
		}
	}

	// Determine yield time.
	yieldMs := defaultWriteStdinYieldMs
	if args.YieldTimeMs != nil {
		yieldMs = *args.YieldTimeMs
	}
	// Empty-input polls enforce a minimum yield to prevent busy polling.
	if args.Chars == "" && yieldMs < emptyInputMinYieldMs {
		yieldMs = emptyInputMinYieldMs
	}
	yieldMs = clamp(yieldMs, minWriteStdinYieldMs, maxWriteStdinYieldMs)

	// Wait for process exit or yield timeout.
	yieldDuration := time.Duration(yieldMs) * time.Millisecond
	sess.Wait(yieldDuration)

	out := execCommandOutput{
		Output:          sess.Output(),
		WallTimeSeconds: sess.WallTime(),
	}
	if sess.Exited() {
		code := sess.ExitCode()
		out.ExitCode = &code
	} else {
		out.SessionID = sess.ID
	}

	data, err := json.Marshal(out)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: marshal output: %v", err), IsError: true}
	}

	isError := out.ExitCode != nil && *out.ExitCode != 0
	return agent.ToolResult{Content: string(data), IsError: isError}
}
