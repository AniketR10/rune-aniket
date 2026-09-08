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

package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"iter"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"google.golang.org/genai"
)

// errEmptyCompletion is surfaced when Gemini ends a turn with no text,
// reasoning, or tool calls, so the agent loop reports a failure instead of
// silently terminating with an empty result.
var errEmptyCompletion = errors.New("gemini returned an empty completion")

// streamIterator bridges the genai pull-style response stream into the
// llmapi.Event iterator contract. Each response chunk may carry several parts;
// they are drained into a queue so Next returns exactly one event at a time.
type streamIterator struct {
	next func() (*genai.GenerateContentResponse, error, bool)
	stop func()

	pendingWarnings []llmapi.Event
	warningIdx      int

	queue    []llmapi.Event
	queueIdx int

	textContent      strings.Builder
	reasoningContent strings.Builder
	toolCalls        []llmapi.ToolCall
	toolCallSeq      int

	// pendingSignature holds a thought_signature seen on a non-functionCall
	// part (Gemini may stream it on an empty-text part preceding the call).
	// It is consumed by the next functionCall part that lacks its own.
	pendingSignature []byte

	usage        llmapi.Usage
	finishReason genai.FinishReason

	done bool
	err  error
}

// newStreamIterator wraps a genai response sequence, pulling values lazily.
// iter.Pull2 runs the producer without spawning a goroutine.
func newStreamIterator(
	seq iter.Seq2[*genai.GenerateContentResponse, error], warnings []llmapi.Event,
) *streamIterator {
	next, stop := iter.Pull2(seq)
	return &streamIterator{next: next, stop: stop, pendingWarnings: warnings}
}

func (s *streamIterator) Next(_ context.Context) (llmapi.Event, bool) {
	if s.warningIdx < len(s.pendingWarnings) {
		ev := s.pendingWarnings[s.warningIdx]
		s.warningIdx++
		return ev, true
	}

	for {
		if s.queueIdx < len(s.queue) {
			ev := s.queue[s.queueIdx]
			s.queueIdx++
			return ev, true
		}
		if s.done {
			return llmapi.Event{}, false
		}

		resp, err, ok := s.next()
		if err != nil {
			s.done = true
			s.err = mapError(err)
			return llmapi.Event{Type: llmapi.EventStreamError, Error: s.err}, true
		}
		if !ok {
			s.done = true
			return s.buildDoneEvent(), true
		}

		s.queue = s.queue[:0]
		s.queueIdx = 0
		s.consume(resp)
	}
}

// consume drains a single response chunk into the event queue and accumulates
// the assistant message + usage for the terminal done event.
func (s *streamIterator) consume(resp *genai.GenerateContentResponse) {
	if resp.UsageMetadata != nil {
		s.usage = usageFromMetadata(resp.UsageMetadata)
	}
	if len(resp.Candidates) == 0 {
		return
	}
	cand := resp.Candidates[0]
	if cand.FinishReason != "" {
		s.finishReason = cand.FinishReason
	}
	if cand.Content == nil {
		return
	}
	for _, part := range cand.Content.Parts {
		s.consumePart(part)
	}
}

func (s *streamIterator) consumePart(part *genai.Part) {
	switch {
	case part.FunctionCall != nil:
		tc := s.toolCallFromPart(part.FunctionCall)
		// Gemini attaches the thought_signature either directly to the
		// functionCall part or to a sibling part streamed just before it.
		sig := part.ThoughtSignature
		if len(sig) == 0 {
			sig = s.pendingSignature
		}
		s.pendingSignature = nil
		setThoughtSignature(&tc, sig)
		s.toolCalls = append(s.toolCalls, tc)
		call := tc
		s.queue = append(s.queue, llmapi.Event{
			Type:     llmapi.EventToolCallDone,
			ToolCall: &call,
		})

	case part.Thought && part.Text != "":
		s.reasoningContent.WriteString(part.Text)
		if len(part.ThoughtSignature) > 0 {
			s.pendingSignature = part.ThoughtSignature
		}
		s.queue = append(s.queue, llmapi.Event{
			Type:      llmapi.EventReasoningDelta,
			Reasoning: part.Text,
		})

	case part.Text != "":
		s.textContent.WriteString(part.Text)
		if len(part.ThoughtSignature) > 0 {
			s.pendingSignature = part.ThoughtSignature
		}
		s.queue = append(s.queue, llmapi.Event{
			Type: llmapi.EventTextDelta,
			Text: part.Text,
		})

	case len(part.ThoughtSignature) > 0:
		// Empty-text part carrying only a signature: hold it for the next
		// functionCall part. Gemini streams this shape before tool calls.
		s.pendingSignature = part.ThoughtSignature
	}
}

// toolCallFromPart maps a Gemini FunctionCall to an llmapi.ToolCall. Gemini
// does not always populate a call ID; synthesize a stable, per-stream-unique
// ID so the agent loop can join the eventual tool result back to this call.
func (s *streamIterator) toolCallFromPart(fc *genai.FunctionCall) llmapi.ToolCall {
	id := fc.ID
	if id == "" {
		s.toolCallSeq++
		id = fmt.Sprintf("call_%d", s.toolCallSeq)
	}
	args := "{}"
	if len(fc.Args) > 0 {
		if b, err := json.Marshal(fc.Args); err == nil {
			args = string(b)
		}
	}
	return llmapi.ToolCall{
		ID:   id,
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionCall{
			Name:      fc.Name,
			Arguments: args,
		},
	}
}

func (s *streamIterator) buildDoneEvent() llmapi.Event {
	// Gemini occasionally ends a turn with no text, reasoning, or tool calls
	// (an empty STOP), commonly when reasoning context has degraded mid-turn.
	// Surfacing this as an error prevents the agent loop — and a parent
	// awaiting a sub-agent — from silently terminating with no result.
	if s.textContent.Len() == 0 && s.reasoningContent.Len() == 0 && len(s.toolCalls) == 0 {
		s.err = errEmptyCompletion
		return llmapi.Event{Type: llmapi.EventStreamError, Error: s.err}
	}
	msg := llmapi.Message{
		Role:             llmapi.RoleAssistant,
		Content:          s.textContent.String(),
		ReasoningContent: s.reasoningContent.String(),
		ToolCalls:        s.toolCalls,
	}
	return llmapi.Event{
		Type: llmapi.EventStreamDone,
		DoneData: &llmapi.DoneData{
			Message:      msg,
			FinishReason: mapFinishReason(s.finishReason, len(s.toolCalls) > 0),
			Usage:        s.usage,
		},
	}
}

func (s *streamIterator) Err() error {
	return s.err
}

func (s *streamIterator) Close() error {
	s.stop()
	return nil
}
