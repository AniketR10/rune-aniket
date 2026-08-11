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
	"fmt"
	"io"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/go-tui/cmd/rune-agent/agent"
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
