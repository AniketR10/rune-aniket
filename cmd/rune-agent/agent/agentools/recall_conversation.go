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

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/memory"
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
