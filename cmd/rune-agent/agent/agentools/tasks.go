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

	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/taskstore"
	"unstable.build/go-tui/cmd/rune-agent/llm"
)

// NewTaskTools returns the four task tracking tools: TaskCreate, TaskUpdate,
// TaskGet, TaskList. All share the same store and progress updater.
func NewTaskTools(store *taskstore.Store, updater agent.ProgressUpdater) []agent.Tool {
	return []agent.Tool{
		&taskCreateTool{store: store, updater: updater},
		&taskUpdateTool{store: store, updater: updater},
		&taskGetTool{store: store},
		&taskListTool{store: store},
	}
}

// --- TaskCreate ---

type taskCreateTool struct {
	store   *taskstore.Store
	updater agent.ProgressUpdater
}

type taskCreateArgs struct {
	Subject     string         `json:"subject"`
	Description string         `json:"description"`
	ActiveForm  string         `json:"activeForm"`
	Metadata    map[string]any `json:"metadata"`
}

func (t *taskCreateTool) Definition() llm.Tool {
	return llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name:        "TaskCreate",
			Description: "Create a new task to track progress on a piece of work.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"subject": map[string]any{
						"type":        "string",
						"description": "A brief title for the task.",
					},
					"description": map[string]any{
						"type":        "string",
						"description": "A detailed description of what needs to be done.",
					},
					"activeForm": map[string]any{
						"type":        "string",
						"description": `Present continuous form shown in spinner when in_progress (e.g., "Running tests").`,
					},
					"metadata": map[string]any{
						"type":        "object",
						"description": "Arbitrary metadata to attach to the task.",
					},
				},
				"required":             []string{"subject", "description"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *taskCreateTool) Summary(arguments string) string {
	var args taskCreateArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}
	return args.Subject
}

func (t *taskCreateTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	var args taskCreateArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: invalid arguments: %v", err), IsError: true}
	}
	if args.Subject == "" {
		return agent.ToolResult{Content: "error: subject is required", IsError: true}
	}
	if args.Description == "" {
		return agent.ToolResult{Content: "error: description is required", IsError: true}
	}

	task := t.store.Create(args.Subject, args.Description, args.ActiveForm, args.Metadata)
	t.updater.UpdateTaskProgress(ctx, task)
	return agent.ToolResult{Content: marshalTask(&task)}
}

// --- TaskUpdate ---

type taskUpdateTool struct {
	store   *taskstore.Store
	updater agent.ProgressUpdater
}

type taskUpdateArgs struct {
	TaskID       string         `json:"taskId"`
	Subject      *string        `json:"subject"`
	Description  *string        `json:"description"`
	ActiveForm   *string        `json:"activeForm"`
	Status       *string        `json:"status"`
	AddBlocks    []string       `json:"addBlocks"`
	AddBlockedBy []string       `json:"addBlockedBy"`
	Owner        *string        `json:"owner"`
	Metadata     map[string]any `json:"metadata"`
}

func (t *taskUpdateTool) Definition() llm.Tool {
	return llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name:        "TaskUpdate",
			Description: "Update an existing task's status, subject, or other fields.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"taskId": map[string]any{
						"type":        "string",
						"description": "The ID of the task to update.",
					},
					"subject": map[string]any{
						"type":        "string",
						"description": "New title for the task.",
					},
					"description": map[string]any{
						"type":        "string",
						"description": "New description for the task.",
					},
					"activeForm": map[string]any{
						"type":        "string",
						"description": `New present continuous form for in_progress spinner.`,
					},
					"status": map[string]any{
						"type":        "string",
						"enum":        []any{"pending", "in_progress", "completed", "deleted"},
						"description": "New status for the task.",
					},
					"addBlocks": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Task IDs that this task blocks.",
					},
					"addBlockedBy": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string"},
						"description": "Task IDs that block this task.",
					},
					"owner": map[string]any{
						"type":        "string",
						"description": "Owner of the task.",
					},
					"metadata": map[string]any{
						"type":        "object",
						"description": "Metadata to merge into the task.",
					},
				},
				"required":             []string{"taskId"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *taskUpdateTool) Summary(arguments string) string {
	var args taskUpdateArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}
	if args.Status != nil {
		return fmt.Sprintf("#%s → %s", args.TaskID, *args.Status)
	}
	return fmt.Sprintf("#%s", args.TaskID)
}

func (t *taskUpdateTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	var args taskUpdateArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: invalid arguments: %v", err), IsError: true}
	}
	if args.TaskID == "" {
		return agent.ToolResult{Content: "error: taskId is required", IsError: true}
	}

	task, err := t.store.Update(args.TaskID, taskstore.UpdateOpts{
		Subject:      args.Subject,
		Description:  args.Description,
		ActiveForm:   args.ActiveForm,
		Status:       args.Status,
		AddBlocks:    args.AddBlocks,
		AddBlockedBy: args.AddBlockedBy,
		Owner:        args.Owner,
		Metadata:     args.Metadata,
	})
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: %v", err), IsError: true}
	}
	t.updater.UpdateTaskProgress(ctx, task)
	return agent.ToolResult{Content: marshalTask(&task)}
}

// --- TaskGet ---

type taskGetTool struct {
	store *taskstore.Store
}

type taskGetArgs struct {
	TaskID string `json:"taskId"`
}

func (t *taskGetTool) Definition() llm.Tool {
	return llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name:        "TaskGet",
			Description: "Get the full details of a task by ID.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"taskId": map[string]any{
						"type":        "string",
						"description": "The ID of the task to retrieve.",
					},
				},
				"required":             []string{"taskId"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *taskGetTool) Summary(arguments string) string {
	var args taskGetArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}
	return fmt.Sprintf("#%s", args.TaskID)
}

func (t *taskGetTool) Execute(_ context.Context, arguments string) agent.ToolResult {
	var args taskGetArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: invalid arguments: %v", err), IsError: true}
	}
	if args.TaskID == "" {
		return agent.ToolResult{Content: "error: taskId is required", IsError: true}
	}
	task, err := t.store.Get(args.TaskID)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: %v", err), IsError: true}
	}
	return agent.ToolResult{Content: marshalTask(&task)}
}

// --- TaskList ---

type taskListTool struct {
	store *taskstore.Store
}

func (t *taskListTool) Definition() llm.Tool {
	return llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name:        "TaskList",
			Description: "List all tasks and their statuses.",
			Parameters: map[string]any{
				"type":                 "object",
				"properties":          map[string]any{},
				"additionalProperties": false,
			},
		},
	}
}

func (t *taskListTool) Summary(string) string { return "" }

func (t *taskListTool) Execute(_ context.Context, _ string) agent.ToolResult {
	tasks := t.store.List()
	type summary struct {
		ID      string `json:"id"`
		Subject string `json:"subject"`
		Status  string `json:"status"`
	}
	summaries := make([]summary, len(tasks))
	for i, task := range tasks {
		summaries[i] = summary{ID: task.ID, Subject: task.Subject, Status: task.Status}
	}
	data, _ := json.Marshal(summaries)
	return agent.ToolResult{Content: string(data)}
}

// marshalTask returns the JSON representation of a task.
func marshalTask(t *taskstore.Task) string {
	data, _ := json.Marshal(t)
	return string(data)
}
