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

package headless

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/rune/cmd/rune-agent/agent"
)

// eventRecord is one line of the stdout JSONL stream.
type eventRecord struct {
	Type         string                `json:"type"`
	Timestamp    string                `json:"timestamp"`
	Text         string                `json:"text,omitempty"`
	Reasoning    string                `json:"reasoning,omitempty"`
	ToolCallID   string                `json:"tool_call_id,omitempty"`
	ToolName     string                `json:"tool_name,omitempty"`
	ToolArgs     string                `json:"tool_args,omitempty"`
	ToolOutput   string                `json:"tool_output,omitempty"`
	IsError      bool                  `json:"is_error,omitempty"`
	DurationMS   int64                 `json:"duration_ms,omitempty"`
	FinishReason string                `json:"finish_reason,omitempty"`
	Usage        *llmapi.DialogueUsage `json:"usage,omitempty"`
	Error        string                `json:"error,omitempty"`
}

// trajectoryRecord is the final line of the stdout JSONL stream.
type trajectoryRecord struct {
	Type       string     `json:"type"`
	Trajectory Trajectory `json:"trajectory"`
}

// streamer writes the agent event stream as JSONL while folding the same
// events into an ATIF trajectory. It is used from a single goroutine.
type streamer struct {
	enc     *json.Encoder
	builder *Builder
	now     func() time.Time
}

func newStreamer(out io.Writer, builder *Builder) *streamer {
	return &streamer{
		enc:     json.NewEncoder(out),
		builder: builder,
		now:     time.Now,
	}
}

// instructions records the task prompt as the opening user step and
// echoes it on the stream.
func (s *streamer) instructions(message string) error {
	s.builder.AddUserStep(message)
	return s.enc.Encode(eventRecord{
		Type:      "instructions",
		Timestamp: s.timestamp(),
		Text:      message,
	})
}

func (s *streamer) event(ev agent.Event) error {
	s.builder.AddEvent(ev)
	rec := eventRecord{
		Type:       eventTypeName(ev.Type),
		Timestamp:  s.timestamp(),
		Text:       ev.Text,
		Reasoning:  ev.Reasoning,
		ToolCallID: ev.ToolCallID,
		ToolName:   ev.ToolName,
		ToolArgs:   ev.ToolArgs,
		ToolOutput: ev.ToolOutput,
		IsError:    ev.IsError,
		DurationMS: ev.ToolDuration.Milliseconds(),
	}
	if ev.Type == agent.EventDone {
		rec.FinishReason = string(ev.FinishReason)
	}
	if ev.Type == agent.EventUsageUpdate {
		usage := ev.Usage
		rec.Usage = &usage
	}
	if ev.Error != nil {
		rec.Error = ev.Error.Error()
	}
	return s.enc.Encode(rec)
}

// finish closes the trajectory and writes it as the final stdout line.
func (s *streamer) finish() (Trajectory, error) {
	traj := s.builder.Finish()
	if err := s.enc.Encode(trajectoryRecord{
		Type:       "trajectory",
		Trajectory: traj,
	}); err != nil {
		return traj, fmt.Errorf("write trajectory: %w", err)
	}
	return traj, nil
}

func (s *streamer) timestamp() string {
	return s.now().UTC().Format(time.RFC3339Nano)
}

func eventTypeName(t agent.EventType) string {
	switch t {
	case agent.EventText:
		return "text"
	case agent.EventToolCall:
		return "tool_call"
	case agent.EventToolResult:
		return "tool_result"
	case agent.EventDone:
		return "done"
	case agent.EventError:
		return "error"
	case agent.EventReasoning:
		return "reasoning"
	case agent.EventCompacting:
		return "compacting"
	case agent.EventCompacted:
		return "compacted"
	case agent.EventRateLimitWarning:
		return "rate_limit_warning"
	case agent.EventUsageUpdate:
		return "usage_update"
	case agent.EventInferenceStart:
		return "inference_start"
	case agent.EventInferenceReady:
		return "inference_ready"
	case agent.EventFirstContent:
		return "first_content"
	case agent.EventToolsStart:
		return "tools_start"
	case agent.EventToolsDropped:
		return "tools_dropped"
	case agent.EventMemoryRecall:
		return "memory_recall"
	case agent.EventRefusal:
		return "refusal"
	}
	return fmt.Sprintf("unknown_%d", int(t))
}
