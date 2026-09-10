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

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/cmd/rune-agent/agent"
	"unstable.build/rune/cmd/rune-agent/memory"
)

type recallConversationTool struct {
	exec       workspaceapi.Executor
	memoryPath string
}

type recallConversationArgs struct {
	MemoryID string `json:"memory_id"`
}

// NewRecallConversation returns a tool that fetches the original
// conversation transcript behind a memory ID.
func NewRecallConversation(exec workspaceapi.Executor, memoryPath string) agent.Tool {
	return &recallConversationTool{exec: exec, memoryPath: memoryPath}
}

func (t *recallConversationTool) NeedsDeterministicOrder() bool { return false }

func (t *recallConversationTool) Definition() llmapi.Tool {
	return llmapi.Tool{
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{
			Name: "recall_conversation",
			Description: `Fetch the original conversation transcript that produced a memory.

Use this when the auto-injected memory context (in <memory-context>) is too
terse and you need the full conversation for deeper understanding. Pass the
memory ID shown in brackets (e.g. "[handler-refactor]" → memory_id = "handler-refactor").`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"memory_id": map[string]any{
						"type":        "string",
						"description": "The ID of the memory whose source conversation to fetch.",
					},
				},
				"required":             []string{"memory_id"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *recallConversationTool) Summary(arguments string) string {
	var args recallConversationArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}
	return args.MemoryID
}

func (t *recallConversationTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	var args recallConversationArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("error: invalid arguments: %v", err),
			IsError: true,
		}
	}
	if args.MemoryID == "" {
		return agent.ToolResult{
			Content: "error: memory_id is required",
			IsError: true,
		}
	}

	transcript, err := memory.FetchConversation(ctx, t.exec, t.memoryPath, args.MemoryID)
	if err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("error: %v", err),
			IsError: true,
		}
	}
	if transcript == "" {
		return agent.ToolResult{Content: "Memory not found."}
	}
	return agent.ToolResult{Content: transcript}
}
