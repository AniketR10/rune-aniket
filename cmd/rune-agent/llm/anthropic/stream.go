// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2026 Unstable Build, All Rights Reserved.
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

package anthropic

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	ant "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"unstable.build/go-tui/cmd/rune-agent/llm"
	"unstable.build/go-tui/cmd/rune-agent/llm/ratelimit"
)

type streamState int

const (
	streamStateStreaming streamState = iota
	streamStateEmitDone
	streamStateCheckHeaders
	streamStateDone
)

// streamIterator wraps the Anthropic SSE stream and emits typed llm.Event values.
type streamIterator struct {
	stream *ssestream.Stream[ant.MessageStreamEventUnion]
	state  streamState

	// Accumulated content.
	textContent      strings.Builder
	reasoningContent strings.Builder
	toolCalls        []llm.ToolCall
	// In-flight tool call argument accumulation, keyed by content block index.
	pendingCalls map[int64]*llm.ToolCall

	// Usage from message_start and message_delta events.
	usage      ant.Usage
	usageDelta ant.MessageDeltaUsage

	// Stop reason from message_delta.
	stopReason ant.StopReason

	err error

	// Rate limit support.
	pendingWarnings []llm.Event
	warningIdx      int
	capturedHeaders http.Header

	// Mid-stream retry support.
	newStream        func() *ssestream.Stream[ant.MessageStreamEventUnion]
	midStreamRetries int
	retryEvents      []llm.Event
	retryEventIdx    int
}

func (s *streamIterator) Next(ctx context.Context) (llm.Event, bool) {
	// Emit buffered warnings from retries.
	if s.warningIdx < len(s.pendingWarnings) {
		ev := s.pendingWarnings[s.warningIdx]
		s.warningIdx++
		return ev, true
	}

	// Emit buffered mid-stream retry events.
	if s.retryEventIdx < len(s.retryEvents) {
		ev := s.retryEvents[s.retryEventIdx]
		s.retryEventIdx++
		return ev, true
	}

	for {
		switch s.state {
		case streamStateDone:
			return llm.Event{}, false

		case streamStateCheckHeaders:
			s.state = streamStateDone
			if s.capturedHeaders != nil {
				if warning, ok := ratelimit.CheckAnthropicRateLimitHeaders(s.capturedHeaders); ok {
					return warning, true
				}
			}
			return llm.Event{}, false

		case streamStateEmitDone:
			s.state = streamStateCheckHeaders
			return s.buildDoneEvent(), true

		case streamStateStreaming:
			if !s.stream.Next() {
				if err := s.stream.Err(); err != nil {
					retryable := ratelimit.IsTransientNetworkError(err) || ratelimit.IsRetryableStreamError(err)
					if s.newStream != nil && s.midStreamRetries > 0 && retryable {
						s.midStreamRetries--
						_ = s.stream.Close()

						attempt := ratelimit.MaxMidStreamRetries - s.midStreamRetries
						wait := ratelimit.RetryWait(nil, attempt-1)
						slog.Warn("mid-stream retryable error, retrying",
							"error", err, "attempt", attempt, "wait", wait)

						time.Sleep(wait)
						s.stream = s.newStream()
						s.resetAccumulator()

						var msg string
						if ratelimit.IsTransientNetworkError(err) {
							msg = ratelimit.RetryNetworkMessage(err, wait, attempt, ratelimit.MaxMidStreamRetries)
						} else {
							msg = ratelimit.RetryStreamMessage(err, wait, attempt, ratelimit.MaxMidStreamRetries)
						}
						s.retryEvents = []llm.Event{
							{Type: llm.EventStreamReset},
							{Type: llm.EventRateLimitWarning, RateLimit: &llm.RateLimitInfo{
								WaitDuration: wait,
								Message:      msg,
							}},
						}
						s.retryEventIdx = 1
						return s.retryEvents[0], true
					}

					s.state = streamStateDone
					s.err = err
					slog.Warn("anthropic completion stream error",
						"error", err,
						"accumulated_text_len", s.textContent.Len(),
						"accumulated_reasoning_len", s.reasoningContent.Len(),
						"accumulated_tool_calls", len(s.toolCalls),
					)
					return llm.Event{Type: llm.EventStreamError, Error: err}, true
				}
				// Stream exhausted normally.
				s.state = streamStateCheckHeaders
				return s.buildDoneEvent(), true
			}

			event := s.stream.Current()
			if ev, ok := s.handleStreamEvent(event); ok {
				return ev, true
			}
			continue
		}
	}
}

// handleStreamEvent processes a single SSE event and returns an llm.Event
// if there's something to emit to the caller.
func (s *streamIterator) handleStreamEvent(event ant.MessageStreamEventUnion) (llm.Event, bool) {
	switch event.Type {
	case "message_start":
		msg := event.AsMessageStart()
		s.usage = msg.Message.Usage

	case "message_delta":
		delta := event.AsMessageDelta()
		s.stopReason = delta.Delta.StopReason
		s.usageDelta = delta.Usage

	case "message_stop":
		s.state = streamStateEmitDone

	case "content_block_start":
		ev := event.AsContentBlockStart()
		switch ev.ContentBlock.Type {
		case "tool_use":
			tb := ev.ContentBlock.AsToolUse()
			if s.pendingCalls == nil {
				s.pendingCalls = make(map[int64]*llm.ToolCall)
			}
			s.pendingCalls[ev.Index] = &llm.ToolCall{
				ID:   tb.ID,
				Type: llm.ToolTypeFunction,
				Function: llm.FunctionCall{
					Name: tb.Name,
				},
			}
		}

	case "content_block_delta":
		ev := event.AsContentBlockDelta()
		switch ev.Delta.Type {
		case "text_delta":
			text := ev.Delta.Text
			s.textContent.WriteString(text)
			return llm.Event{Type: llm.EventTextDelta, Text: text}, true

		case "thinking_delta":
			thinking := ev.Delta.Thinking
			s.reasoningContent.WriteString(thinking)
			return llm.Event{Type: llm.EventReasoningDelta, Reasoning: thinking}, true

		case "input_json_delta":
			if tc, ok := s.pendingCalls[ev.Index]; ok {
				tc.Function.Arguments += ev.Delta.PartialJSON
			}
		}

	case "content_block_stop":
		ev := event.AsContentBlockStop()
		if tc, ok := s.pendingCalls[ev.Index]; ok {
			delete(s.pendingCalls, ev.Index)
			s.toolCalls = append(s.toolCalls, *tc)
			return llm.Event{
				Type:     llm.EventToolCallDone,
				ToolCall: tc,
			}, true
		}
	}
	return llm.Event{}, false
}

func (s *streamIterator) buildDoneEvent() llm.Event {
	msg := llm.Message{
		Role:             llm.RoleAssistant,
		Content:          s.textContent.String(),
		ReasoningContent: s.reasoningContent.String(),
		ToolCalls:        s.toolCalls,
	}

	finishReason := mapStopReason(s.stopReason)

	// Combine usage from message_start and message_delta.
	// Anthropic splits input tokens into three buckets: InputTokens (uncached),
	// CacheReadInputTokens, and CacheCreationInputTokens. TokensSent must be
	// the sum of all three so it represents total input tokens, matching the
	// semantics of OpenAI's PromptTokens which already includes cached tokens.
	cached := int(s.usage.CacheReadInputTokens) + int(s.usageDelta.CacheReadInputTokens)
	cacheCreated := int(s.usage.CacheCreationInputTokens) + int(s.usageDelta.CacheCreationInputTokens)
	uncached := int(s.usage.InputTokens) + int(s.usageDelta.InputTokens)

	usage := llm.Usage{
		TokensSent:         uncached + cached + cacheCreated,
		TokensReceived:     int(s.usage.OutputTokens) + int(s.usageDelta.OutputTokens),
		TokensCached:       cached,
		TokensCacheCreated: cacheCreated,
	}

	return llm.Event{
		Type: llm.EventStreamDone,
		DoneData: &llm.DoneData{
			Message:      msg,
			FinishReason: finishReason,
			Usage:        usage,
		},
	}
}

func (s *streamIterator) resetAccumulator() {
	s.textContent.Reset()
	s.reasoningContent.Reset()
	s.toolCalls = nil
	s.pendingCalls = nil
	s.usage = ant.Usage{}
	s.usageDelta = ant.MessageDeltaUsage{}
	s.stopReason = ""
}

func (s *streamIterator) Err() error {
	return s.err
}

func (s *streamIterator) Close() error {
	return s.stream.Close()
}

// mapStopReason converts Anthropic stop reasons to llm.FinishReason.
func mapStopReason(reason ant.StopReason) llm.FinishReason {
	switch reason {
	case ant.StopReasonEndTurn:
		return llm.FinishReasonStop
	case ant.StopReasonToolUse:
		return llm.FinishReasonToolCall
	case ant.StopReasonMaxTokens:
		return llm.FinishReasonLength
	case ant.StopReasonStopSequence:
		return llm.FinishReasonStop
	default:
		return llm.FinishReasonNull
	}
}
