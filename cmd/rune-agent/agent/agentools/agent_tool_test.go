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
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
	"unstable.build/go-tui/cmd/rune-agent/llm"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

func TestAgentTool(t *testing.T) {
	t.Run("successful run returns reply", func(t *testing.T) {
		spawner := &mockSpawner{
			handle: agent.RunHandle{
				SessionKey: "sk-123",
				Label:      "search code",
				Events:     newMockEventIterator(agent.Event{Type: agent.EventText, Text: "result"}),
			},
		}
		childEvents := make(chan agent.ChildEvent, 64)
		tool := NewAgentTool(spawner, nil, childEvents, nil)
		ctx := agent.WithParentToolCallID(context.Background(), "parent-1")
		result := tool.Execute(
			ctx,
			`{"description":"search code","prompt":"find all tests"}`,
		)

		assert.False(t, result.IsError)
		assert.Equal(t, "result", result.Content)
	})

	t.Run("run error", func(t *testing.T) {
		spawner := &mockSpawner{
			runErr: errors.New("not allowed"),
		}
		tool := NewAgentTool(spawner, nil, nil, nil)
		result := tool.Execute(
			context.Background(),
			`{"description":"do work","prompt":"do work"}`,
		)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "not allowed")
	})

	t.Run("invalid JSON", func(t *testing.T) {
		tool := NewAgentTool(&mockSpawner{}, nil, nil, nil)
		result := tool.Execute(
			context.Background(), `not json`,
		)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content,
			"invalid arguments")
	})

	t.Run("empty prompt", func(t *testing.T) {
		tool := NewAgentTool(&mockSpawner{}, nil, nil, nil)
		result := tool.Execute(
			context.Background(),
			`{"description":"x","prompt":""}`,
		)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content,
			"prompt is required")
	})

	t.Run("definition", func(t *testing.T) {
		tool := NewAgentTool(&mockSpawner{}, nil, nil, nil)
		def := tool.Definition()

		assert.Equal(t, llm.ToolTypeFunction, def.Type)
		assert.Equal(t, "agent", def.Function.Name)
		assert.NotEmpty(t, def.Function.Description)
		assert.NotNil(t, def.Function.Parameters)
	})

	t.Run("description includes agent types", func(t *testing.T) {
		agents := []agent.AgentSummary{
			{ID: "coder", Name: "Coder Agent"},
			{ID: "reviewer", Name: "Review Agent"},
		}
		tool := NewAgentTool(&mockSpawner{}, agents, nil, nil)
		def := tool.Definition()

		assert.Contains(t, def.Function.Description,
			"coder: Coder Agent")
		assert.Contains(t, def.Function.Description,
			"reviewer: Review Agent")
		assert.Contains(t, def.Function.Description,
			"subagent_type")
	})

	t.Run("description without agents omits types section",
		func(t *testing.T) {
			tool := NewAgentTool(&mockSpawner{}, nil, nil, nil)
			def := tool.Definition()

			assert.NotContains(t,
				def.Function.Description,
				"Available agent types")
		},
	)

	t.Run("description includes agent-type skills from registry",
		func(t *testing.T) {
			dir := t.TempDir()
			skillDir := filepath.Join(dir, "my-planner")
			require.NoError(t, os.MkdirAll(skillDir, 0o755))
			require.NoError(t, os.WriteFile(
				filepath.Join(skillDir, "SKILL.md"),
				[]byte("---\nname: my-planner\ndescription: Plans things\ntype: agent\n---\nbody"),
				0o644,
			))
			reg := skills.NewRegistry(localFS{}, dirURI(""), []string{dir}, nil)

			tool := NewAgentTool(&mockSpawner{}, nil, nil, reg)
			def := tool.Definition()

			assert.Contains(t, def.Function.Description,
				"my-planner: Plans things")
			assert.Contains(t, def.Function.Description,
				"Available agent types")
		},
	)

	t.Run("description combines agents and agent-type skills",
		func(t *testing.T) {
			dir := t.TempDir()
			skillDir := filepath.Join(dir, "my-planner")
			require.NoError(t, os.MkdirAll(skillDir, 0o755))
			require.NoError(t, os.WriteFile(
				filepath.Join(skillDir, "SKILL.md"),
				[]byte("---\nname: my-planner\ndescription: Plans things\ntype: agent\n---\nbody"),
				0o644,
			))
			reg := skills.NewRegistry(localFS{}, dirURI(""), []string{dir}, nil)

			agents := []agent.AgentSummary{
				{ID: "coder", Name: "Coder Agent"},
			}
			tool := NewAgentTool(&mockSpawner{}, agents, nil, reg)
			def := tool.Definition()

			assert.Contains(t, def.Function.Description, "coder: Coder Agent")
			assert.Contains(t, def.Function.Description, "my-planner: Plans things")
		},
	)

	t.Run("only description and prompt are required",
		func(t *testing.T) {
			tool := NewAgentTool(&mockSpawner{}, nil, nil, nil)
			def := tool.Definition()
			params := def.Function.Parameters.(map[string]any)
			required := params["required"].([]string)

			assert.Equal(t,
				[]string{"description", "prompt"},
				required,
			)
		},
	)

	t.Run("passes all parameters to spawner", func(t *testing.T) {
		spawner := &mockSpawner{
			handle: agent.RunHandle{
				Events: newMockEventIterator(),
			},
		}
		tool := NewAgentTool(spawner, nil, nil, nil)
		tool.Execute(
			context.Background(),
			`{
				"description":"search code",
				"prompt":"find all tests",
				"subagent_type":"coder",
				"model":"gpt-4",
				"timeoutSeconds":30,
				"cleanup":"delete"
			}`,
		)

		req := spawner.lastRunReq
		assert.Equal(t, "find all tests", req.Message)
		assert.Equal(t, "search code", req.Label)
		assert.Equal(t, "coder", req.AgentID)
		assert.Equal(t, "gpt-4", req.Model)
		assert.Equal(t, 30, req.TimeoutSeconds)
		assert.Equal(t, "delete", req.Cleanup)
	})

	t.Run("reasoning-only response is captured", func(t *testing.T) {
		spawner := &mockSpawner{
			handle: agent.RunHandle{
				Events: newMockEventIterator(
					agent.Event{Type: agent.EventReasoning, Reasoning: "The answer is 42."},
				),
			},
		}
		tool := NewAgentTool(spawner, nil, nil, nil)
		result := tool.Execute(
			context.Background(),
			`{"description":"think","prompt":"what is the answer?"}`,
		)

		assert.False(t, result.IsError)
		assert.Equal(t, "The answer is 42.", result.Content,
			"reasoning-only sub-agent output must not be lost")
	})

	t.Run("text preferred over reasoning", func(t *testing.T) {
		spawner := &mockSpawner{
			handle: agent.RunHandle{
				Events: newMockEventIterator(
					agent.Event{Type: agent.EventReasoning, Reasoning: "thinking..."},
					agent.Event{Type: agent.EventText, Text: "final answer"},
				),
			},
		}
		tool := NewAgentTool(spawner, nil, nil, nil)
		result := tool.Execute(
			context.Background(),
			`{"description":"think","prompt":"what is the answer?"}`,
		)

		assert.False(t, result.IsError)
		assert.Equal(t, "final answer", result.Content,
			"text output must be preferred over reasoning")
	})

	t.Run("sub-agent error is propagated", func(t *testing.T) {
		spawner := &mockSpawner{
			handle: agent.RunHandle{
				Events: newMockEventIterator(
					agent.Event{
						Type:  agent.EventError,
						Error: errors.New("get dialogue: context canceled"),
					},
				),
			},
		}
		tool := NewAgentTool(spawner, nil, nil, nil)
		result := tool.Execute(
			context.Background(),
			`{"description":"research","prompt":"do research"}`,
		)

		assert.True(t, result.IsError, "sub-agent error must be reported as error")
		assert.Contains(t, result.Content, "context canceled",
			"sub-agent error message must be in tool result content")
	})

	t.Run("forwards child events", func(t *testing.T) {
		spawner := &mockSpawner{
			handle: agent.RunHandle{
				Events: newMockEventIterator(
					agent.Event{Type: agent.EventToolCall, ToolCallID: "tc1", ToolName: "read_file"},
					agent.Event{Type: agent.EventToolResult, ToolCallID: "tc1"},
					agent.Event{Type: agent.EventText, Text: "done"},
				),
			},
		}
		childEvents := make(chan agent.ChildEvent, 64)
		tool := NewAgentTool(spawner, nil, childEvents, nil)
		ctx := agent.WithParentToolCallID(context.Background(), "parent-42")
		result := tool.Execute(ctx, `{"description":"test","prompt":"test"}`)

		assert.False(t, result.IsError)
		assert.Equal(t, "done", result.Content)

		// Drain child events
		close(childEvents)
		var events []agent.ChildEvent
		for ev := range childEvents {
			events = append(events, ev)
		}
		assert.NotEmpty(t, events)
		for _, ev := range events {
			assert.Equal(t, "parent-42", ev.ParentToolCallID)
		}
	})
}

var _ agent.Tool = (*agentTool)(nil)

type mockSpawner struct {
	handle     agent.RunHandle
	runErr     error
	lastRunReq agent.RunRequest
}

func (m *mockSpawner) Run(
	_ context.Context, req agent.RunRequest,
) (agent.RunHandle, error) {
	m.lastRunReq = req
	return m.handle, m.runErr
}

// mockEventIterator is a simple iterator that yields preset events.
type mockEventIterator struct {
	events []agent.Event
	idx    int
	err    error
}

func newMockEventIterator(events ...agent.Event) iterator.Iterator[agent.Event] {
	return &mockEventIterator{events: events}
}

func (m *mockEventIterator) Next(_ context.Context) (agent.Event, bool) {
	if m.idx >= len(m.events) {
		return agent.Event{}, false
	}
	ev := m.events[m.idx]
	m.idx++
	// Mirror channelIterator: set err on EventError.
	if ev.Type == agent.EventError {
		m.err = ev.Error
	}
	return ev, true
}

func (m *mockEventIterator) Err() error   { return m.err }
func (m *mockEventIterator) Close() error { return nil }
