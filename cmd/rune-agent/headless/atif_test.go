// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package headless

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/go-tui/cmd/rune-agent/agent"
)

// fixedClock keeps golden output stable.
func fixedClock() func() time.Time {
	ts := time.Date(2026, 8, 11, 8, 52, 33, 0, time.UTC)
	return func() time.Time { return ts }
}

func newTestBuilder(t *testing.T) *Builder {
	t.Helper()
	b := NewBuilder(BuilderConfig{
		AgentVersion:    "v1.2.3",
		SessionID:       "run-1",
		ModelName:       "claude-opus-4-6",
		ReasoningEffort: "high",
	})
	b.now = fixedClock()
	return b
}

func TestBuilderStepSequencing(t *testing.T) {
	b := newTestBuilder(t)
	b.AddUserStep("do the thing")
	b.AddEvent(agent.Event{Type: agent.EventText, Text: "on it"})
	b.AddEvent(agent.Event{Type: agent.EventUsageUpdate})
	b.AddEvent(agent.Event{Type: agent.EventText, Text: "done"})
	b.AddEvent(agent.Event{Type: agent.EventUsageUpdate})

	traj := b.Finish()

	require.Len(t, traj.Steps, 3)
	for i, s := range traj.Steps {
		assert.Equal(t, i+1, s.StepID)
	}
	assert.Equal(t, SchemaVersion, traj.SchemaVersion)
	assert.Equal(t, "rune-agent", traj.Agent.Name)
	assert.Equal(t, 3, traj.FinalMetrics.TotalSteps)
}

func TestBuilderOmitsAgentFieldsOnNonAgentSteps(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(b *Builder)
		index int
	}{
		{
			name:  "user step",
			build: func(b *Builder) { b.AddUserStep("hello") },
			index: 0,
		},
		{
			name: "refusal system step",
			build: func(b *Builder) {
				b.AddEvent(agent.Event{Type: agent.EventRefusal})
			},
			index: 0,
		},
		{
			name: "error system step",
			build: func(b *Builder) {
				b.AddEvent(agent.Event{
					Type: agent.EventError, Error: errors.New("boom")})
			},
			index: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newTestBuilder(t)
			tc.build(b)

			step := b.Finish().Steps[tc.index]

			assert.Empty(t, step.ModelName)
			assert.Empty(t, step.ReasoningEffort)
			assert.Empty(t, step.ReasoningContent)
			assert.Empty(t, step.ToolCalls)
			assert.Nil(t, step.Metrics)
			assert.Zero(t, step.LLMCallCount)
		})
	}
}

func TestBuilderTerminalStatus(t *testing.T) {
	for _, tc := range []struct {
		name   string
		event  agent.Event
		status string
		msg    string
	}{
		{
			name:   "refusal",
			event:  agent.Event{Type: agent.EventRefusal},
			status: "refused",
			msg:    "the model refused to continue",
		},
		{
			name:   "error",
			event:  agent.Event{Type: agent.EventError, Error: errors.New("boom")},
			status: "error",
			msg:    "boom",
		},
		{
			name:   "error without cause",
			event:  agent.Event{Type: agent.EventError},
			status: "error",
			msg:    "the agent loop terminated with an error",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := newTestBuilder(t)
			b.AddUserStep("go")
			b.AddEvent(agent.Event{Type: agent.EventText, Text: "trying"})
			b.AddEvent(tc.event)

			traj := b.Finish()

			last := traj.Steps[len(traj.Steps)-1]
			assert.Equal(t, SourceSystem, last.Source)
			assert.Equal(t, tc.msg, last.Message)
			assert.Equal(t, tc.status, traj.Extra["status"])
			// The partial agent turn is preserved ahead of the
			// system step.
			assert.Equal(t, "trying", traj.Steps[1].Message)
			assert.Equal(t, SourceAgent, traj.Steps[1].Source)
		})
	}
}

func TestBuilderReasoningAndToolCalls(t *testing.T) {
	b := newTestBuilder(t)
	b.AddEvent(agent.Event{Type: agent.EventReasoning, Reasoning: "think "})
	b.AddEvent(agent.Event{Type: agent.EventReasoning, Reasoning: "harder"})
	b.AddEvent(agent.Event{Type: agent.EventText, Text: "reading"})
	b.AddEvent(agent.Event{
		Type:       agent.EventToolCall,
		ToolCallID: "call-1",
		ToolName:   "read_file",
		ToolArgs:   `{"path":"main.go"}`,
	})
	b.AddEvent(agent.Event{Type: agent.EventUsageUpdate})

	step := b.Finish().Steps[0]

	assert.Equal(t, SourceAgent, step.Source)
	assert.Equal(t, "claude-opus-4-6", step.ModelName)
	assert.Equal(t, "high", step.ReasoningEffort)
	assert.Equal(t, "think harder", step.ReasoningContent)
	assert.Equal(t, "reading", step.Message)
	assert.Equal(t, 1, step.LLMCallCount)
	require.Len(t, step.ToolCalls, 1)
	assert.Equal(t, "call-1", step.ToolCalls[0].ToolCallID)
	assert.Equal(t, "read_file", step.ToolCalls[0].FunctionName)
	assert.Equal(t,
		map[string]any{"path": "main.go"}, step.ToolCalls[0].Arguments)
}

func TestBuilderGolden(t *testing.T) {
	b := NewBuilder(BuilderConfig{
		AgentVersion:    "v1.2.3",
		SessionID:       "run-1",
		ModelName:       "claude-opus-4-6",
		ReasoningEffort: "high",
		ToolDefinitions: []llmapi.Tool{{
			Type: "function",
			Function: llmapi.FunctionDefinition{
				Name:        "read_file",
				Description: "Read a file.",
				Parameters:  map[string]any{"type": "object"},
			},
		}},
		Extra: map[string]any{"workspace": "file:///w"},
	})
	b.now = fixedClock()

	b.AddUserStep("read main.go")
	b.AddEvent(agent.Event{Type: agent.EventText, Text: "reading"})
	b.AddEvent(agent.Event{
		Type:       agent.EventToolCall,
		ToolCallID: "call-1",
		ToolName:   "read_file",
		ToolArgs:   `{"path":"main.go"}`,
	})
	b.AddEvent(agent.Event{
		Type: agent.EventUsageUpdate,
		Usage: llmapi.DialogueUsage{
			TokensSent: 160, TokensReceived: 20, TokensCached: 50,
		},
	})
	b.AddEvent(agent.Event{
		Type:       agent.EventToolResult,
		ToolCallID: "call-1",
		ToolName:   "read_file",
		ToolOutput: "package main",
	})
	b.AddEvent(agent.Event{Type: agent.EventText, Text: "all done"})
	b.AddEvent(agent.Event{
		Type: agent.EventUsageUpdate,
		Usage: llmapi.DialogueUsage{
			TokensSent: 400, TokensReceived: 35, TokensCached: 150,
			TokensCacheCreated: 90,
		},
	})

	got, err := json.MarshalIndent(b.Finish(), "", "  ")
	require.NoError(t, err)
	assert.JSONEq(t, goldenTrajectory, string(got))
}

const goldenTrajectory = `{
  "schema_version": "ATIF-v1.7",
  "session_id": "run-1",
  "agent": {
    "name": "rune-agent",
    "version": "v1.2.3",
    "model_name": "claude-opus-4-6",
    "tool_definitions": [
      {
        "type": "function",
        "function": {
          "name": "read_file",
          "description": "Read a file.",
          "parameters": {"type": "object"}
        }
      }
    ]
  },
  "steps": [
    {
      "step_id": 1,
      "timestamp": "2026-08-11T08:52:33Z",
      "source": "user",
      "message": "read main.go"
    },
    {
      "step_id": 2,
      "timestamp": "2026-08-11T08:52:33Z",
      "source": "agent",
      "model_name": "claude-opus-4-6",
      "reasoning_effort": "high",
      "message": "reading",
      "tool_calls": [
        {
          "tool_call_id": "call-1",
          "function_name": "read_file",
          "arguments": {"path": "main.go"}
        }
      ],
      "observation": {
        "results": [
          {"source_call_id": "call-1", "content": "package main"}
        ]
      },
      "metrics": {
        "prompt_tokens": 160,
        "completion_tokens": 20,
        "cached_tokens": 50
      },
      "llm_call_count": 1
    },
    {
      "step_id": 3,
      "timestamp": "2026-08-11T08:52:33Z",
      "source": "agent",
      "model_name": "claude-opus-4-6",
      "reasoning_effort": "high",
      "message": "all done",
      "metrics": {
        "prompt_tokens": 240,
        "completion_tokens": 15,
        "cached_tokens": 100,
        "extra": {"cache_creation_input_tokens": 90}
      },
      "llm_call_count": 1
    }
  ],
  "final_metrics": {
    "total_prompt_tokens": 400,
    "total_completion_tokens": 35,
    "total_cached_tokens": 150,
    "total_steps": 3
  },
  "extra": {"workspace": "file:///w"}
}`
