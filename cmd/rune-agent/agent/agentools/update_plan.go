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
	"sync"

	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/taskstore"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

// updatePlanTool is a Codex-style checklist/progress tool. It updates the
// chat UI's plan checklist without invoking plan approval / clear-context flow.
type updatePlanTool struct {
	updater agent.ProgressUpdater
	store   *taskstore.Store
	mu      sync.Mutex
	stepIDs map[string]string
}

type updatePlanArgs struct {
	Explanation string     `json:"explanation"`
	Plan        []planStep `json:"plan"`
}

type planStep struct {
	Step   string `json:"step"`
	Status string `json:"status"`
}

// NewUpdatePlan creates an update_plan tool with the Codex schema.
// It mirrors Codex semantics by updating checklist/progress UI state.
func NewUpdatePlan(updater agent.ProgressUpdater) agent.Tool {
	return &updatePlanTool{
		updater: updater,
		store:   taskstore.New(),
		stepIDs: make(map[string]string),
	}
}

func (t *updatePlanTool) NeedsDeterministicOrder() bool { return false }

func (t *updatePlanTool) Definition() llmapi.Tool {
	return llmapi.Tool{
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{
			Name: "update_plan",
			Description: `Updates the task plan.
Provide an optional explanation and a list of plan items, each with a step and status.
At most one step can be in_progress at a time.
`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"explanation": map[string]any{
						"type": "string",
					},
					"plan": map[string]any{
						"type":        "array",
						"description": "The list of steps",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"step": map[string]any{
									"type": "string",
								},
								"status": map[string]any{
									"type":        "string",
									"description": "One of: pending, in_progress, completed",
								},
							},
							"required":             []string{"step", "status"},
							"additionalProperties": false,
						},
					},
				},
				"required":             []string{"plan"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *updatePlanTool) Summary(arguments string) string {
	var args updatePlanArgs
	if json.Unmarshal([]byte(arguments), &args) != nil {
		return ""
	}
	s := args.Explanation
	if s == "" && len(args.Plan) > 0 {
		s = args.Plan[0].Step
	}
	if len(s) > 60 {
		s = s[:60] + "..."
	}
	return s
}

func (t *updatePlanTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	var args updatePlanArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf("invalid arguments: %v", err),
			IsError: true,
		}
	}
	if len(args.Plan) == 0 {
		return agent.ToolResult{
			Content: "invalid arguments: plan must have at least one step",
			IsError: true,
		}
	}

	inProgress := 0
	for _, step := range args.Plan {
		switch step.Status {
		case "pending", "in_progress", "completed":
			if step.Status == "in_progress" {
				inProgress++
			}
		default:
			return agent.ToolResult{
				Content: fmt.Sprintf("invalid arguments: unknown status %q", step.Status),
				IsError: true,
			}
		}
	}
	if inProgress > 1 {
		return agent.ToolResult{
			Content: "invalid arguments: at most one step can be in_progress",
			IsError: true,
		}
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	seen := make(map[string]bool, len(args.Plan))
	for _, step := range args.Plan {
		id, ok := t.stepIDs[step.Step]
		if !ok {
			task := t.store.Create(step.Step, "", "", nil)
			id = task.ID
			t.stepIDs[step.Step] = id
		}
		status := step.Status
		task, err := t.store.Update(id, taskstore.UpdateOpts{Status: &status})
		if err != nil {
			return agent.ToolResult{
				Content: fmt.Sprintf("update plan task %q: %v", step.Step, err),
				IsError: true,
			}
		}
		t.updater.UpdateTaskProgress(ctx, task)
		seen[step.Step] = true
	}

	for step, id := range t.stepIDs {
		if seen[step] {
			continue
		}
		status := "deleted"
		task, err := t.store.Update(id, taskstore.UpdateOpts{Status: &status})
		if err != nil {
			return agent.ToolResult{
				Content: fmt.Sprintf("delete plan task %q: %v", step, err),
				IsError: true,
			}
		}
		t.updater.UpdateTaskProgress(ctx, task)
		delete(t.stepIDs, step)
	}

	return agent.ToolResult{Content: "Plan updated"}
}
