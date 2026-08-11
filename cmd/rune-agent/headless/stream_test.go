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
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/go-tui/cmd/rune-agent/agent"
)

func TestDecodeArguments(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want map[string]any
	}{
		{name: "object", raw: `{"a":1}`, want: map[string]any{"a": float64(1)}},
		{name: "empty", raw: "", want: map[string]any{}},
		{name: "malformed", raw: "{oops", want: map[string]any{"_raw": "{oops"}},
		{name: "array", raw: `[1,2]`, want: map[string]any{"_raw": "[1,2]"}},
		{name: "null", raw: "null", want: map[string]any{"_raw": "null"}},
		{name: "scalar", raw: `"hi"`, want: map[string]any{"_raw": `"hi"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, decodeArguments(tc.raw))
		})
	}
}

func TestBuilderDifferencesCumulativeUsage(t *testing.T) {
	b := newTestBuilder(t)
	usages := []llmapi.DialogueUsage{
		{TokensSent: 100, TokensReceived: 10, TokensCached: 40, TokensCacheCreated: 25},
		{TokensSent: 260, TokensReceived: 25, TokensCached: 90, TokensCacheCreated: 25},
		{TokensSent: 300, TokensReceived: 31, TokensCached: 90, TokensCacheCreated: 60},
	}
	for _, u := range usages {
		b.AddEvent(agent.Event{Type: agent.EventText, Text: "turn"})
		b.AddEvent(agent.Event{Type: agent.EventUsageUpdate, Usage: u})
	}

	steps := b.Finish().Steps

	require.Len(t, steps, 3)
	assert.Equal(t, &Metrics{
		PromptTokens: 100, CompletionTokens: 10, CachedTokens: 40,
		Extra: map[string]any{"cache_creation_input_tokens": 25},
	}, steps[0].Metrics)
	assert.Equal(t, &Metrics{
		PromptTokens: 160, CompletionTokens: 15, CachedTokens: 50,
	}, steps[1].Metrics)
	assert.Equal(t, &Metrics{
		PromptTokens: 40, CompletionTokens: 6, CachedTokens: 0,
		Extra: map[string]any{"cache_creation_input_tokens": 35},
	}, steps[2].Metrics)
}

func TestBuilderAttachesObservationsRetroactively(t *testing.T) {
	b := newTestBuilder(t)
	b.AddEvent(agent.Event{
		Type: agent.EventToolCall, ToolCallID: "a", ToolName: "read_file"})
	b.AddEvent(agent.Event{Type: agent.EventUsageUpdate})
	b.AddEvent(agent.Event{
		Type: agent.EventToolResult, ToolCallID: "a", ToolOutput: "first"})
	// A second turn opens before the previous turn's result is
	// consumed, so the writer must not attach it to the newer step.
	b.AddEvent(agent.Event{
		Type: agent.EventToolCall, ToolCallID: "b", ToolName: "bash"})
	b.AddEvent(agent.Event{Type: agent.EventUsageUpdate})
	b.AddEvent(agent.Event{
		Type: agent.EventToolResult, ToolCallID: "b",
		ToolOutput: "second", IsError: true})

	steps := b.Finish().Steps

	require.Len(t, steps, 2)
	require.NotNil(t, steps[0].Observation)
	assert.Equal(t, []ObservationResult{
		{SourceCallID: "a", Content: "first"},
	}, steps[0].Observation.Results)
	require.NotNil(t, steps[1].Observation)
	assert.Equal(t, []ObservationResult{
		{SourceCallID: "b", Content: "second",
			Extra: map[string]any{"is_error": true}},
	}, steps[1].Observation.Results)

	// Every source_call_id resolves within its own step.
	for _, s := range steps {
		ids := make(map[string]bool, len(s.ToolCalls))
		for _, c := range s.ToolCalls {
			ids[c.ToolCallID] = true
		}
		for _, r := range s.Observation.Results {
			assert.True(t, ids[r.SourceCallID],
				"source_call_id %q not in step %d", r.SourceCallID, s.StepID)
		}
	}
}

func TestStreamerEmitsJSONLAndTrajectoryEnvelope(t *testing.T) {
	var buf bytes.Buffer
	b := newTestBuilder(t)
	s := newStreamer(&buf, b)
	s.now = fixedClock()

	require.NoError(t, s.instructions("fix the bug"))
	require.NoError(t, s.event(agent.Event{Type: agent.EventText, Text: "ok"}))
	require.NoError(t, s.event(agent.Event{
		Type: agent.EventToolCall, ToolCallID: "c1",
		ToolName: "bash", ToolArgs: `{"command":"ls"}`}))
	require.NoError(t, s.event(agent.Event{
		Type:  agent.EventUsageUpdate,
		Usage: llmapi.DialogueUsage{TokensSent: 12, TokensReceived: 3}}))
	traj, err := s.finish()
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	require.Len(t, lines, 5)

	var kinds []string
	for _, l := range lines[:4] {
		var rec eventRecord
		require.NoError(t, json.Unmarshal([]byte(l), &rec))
		kinds = append(kinds, rec.Type)
	}
	assert.Equal(t,
		[]string{"instructions", "text", "tool_call", "usage_update"}, kinds)

	var env trajectoryRecord
	require.NoError(t, json.Unmarshal([]byte(lines[4]), &env))
	assert.Equal(t, "trajectory", env.Type)
	assert.Equal(t, traj, env.Trajectory)
	assert.Equal(t, SchemaVersion, env.Trajectory.SchemaVersion)
	require.Len(t, env.Trajectory.Steps, 2)
	assert.Equal(t, SourceUser, env.Trajectory.Steps[0].Source)
	assert.Equal(t, "fix the bug", env.Trajectory.Steps[0].Message)
}
