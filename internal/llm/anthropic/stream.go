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

package anthropic

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	ant "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/rune/internal/llm/ratelimit"
)

type streamState int

const (
	streamStateStreaming streamState = iota
	streamStateEmitDone
	streamStateCheckHeaders
	streamStateDone
)

// streamIterator wraps the Anthropic SSE stream and emits typed llmapi.Event values.
type streamIterator struct {
	stream *ssestream.Stream[ant.MessageStreamEventUnion]
	state  streamState

	// Accumulated content.
	textContent      strings.Builder
	reasoningContent strings.Builder
	toolCalls        []llmapi.ToolCall
	// In-flight tool call argument accumulation, keyed by content block index.
	pendingCalls map[int64]*llmapi.ToolCall

	// Reasoning replay accumulation. reasoningBlocks holds finalized thinking
	// and redacted_thinking blocks in stream order so the assistant turn can
	// be replayed faithfully (thinking blocks carry a byte-exact signature
	// that the API requires on replay). pendingThinking tracks in-flight
	// thinking blocks by content block index until their content_block_stop.
	reasoningBlocks []llmapi.ReasoningBlock
	pendingThinking map[int64]*llmapi.ReasoningBlock

	// Usage from message_start and message_delta events.
	usage      ant.Usage
	usageDelta ant.MessageDeltaUsage

	// Stop reason from message_delta.
	stopReason ant.StopReason

	err error

	// Rate limit support.
	pendingWarnings []llmapi.Event
	warningIdx      int
	capturedHeaders http.Header

	// Mid-stream retry support.
	newStream        func() *ssestream.Stream[ant.MessageStreamEventUnion]
	midStreamRetries int
	retryEvents      []llmapi.Event
	retryEventIdx    int
}

func (s *streamIterator) Next(ctx context.Context) (llmapi.Event, bool) {
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
			return llmapi.Event{}, false

		case streamStateCheckHeaders:
			s.state = streamStateDone
			if s.capturedHeaders != nil {
				if warning, ok := ratelimit.CheckAnthropicRateLimitHeaders(s.capturedHeaders); ok {
					return warning, true
				}
			}
			return llmapi.Event{}, false

		case streamStateEmitDone:
			s.state = streamStateCheckHeaders
			return s.buildDoneEvent(), true

		case streamStateStreaming:
			if !s.stream.Next() {
				if err := s.stream.Err(); err != nil {
					retryable := ratelimit.IsTransientNetworkError(err) || isRetryableMidStreamError(err)
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
						s.retryEvents = []llmapi.Event{
							{Type: llmapi.EventStreamReset},
							{Type: llmapi.EventRateLimitWarning, RateLimit: &llmapi.RateLimitInfo{
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
					return llmapi.Event{Type: llmapi.EventStreamError, Error: err}, true
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

// handleStreamEvent processes a single SSE event and returns an llmapi.Event
// if there's something to emit to the caller.
func (s *streamIterator) handleStreamEvent(event ant.MessageStreamEventUnion) (llmapi.Event, bool) {
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
				s.pendingCalls = make(map[int64]*llmapi.ToolCall)
			}
			s.pendingCalls[ev.Index] = &llmapi.ToolCall{
				ID:   tb.ID,
				Type: llmapi.ToolTypeFunction,
				Function: llmapi.FunctionCall{
					Name: tb.Name,
				},
			}

		case "thinking":
			tb := ev.ContentBlock.AsThinking()
			if s.pendingThinking == nil {
				s.pendingThinking = make(map[int64]*llmapi.ReasoningBlock)
			}
			s.pendingThinking[ev.Index] = &llmapi.ReasoningBlock{
				Kind:      reasoningKindThinking,
				Text:      tb.Thinking,
				Signature: tb.Signature,
			}

		case "redacted_thinking":
			rb := ev.ContentBlock.AsRedactedThinking()
			s.reasoningBlocks = append(s.reasoningBlocks, llmapi.ReasoningBlock{
				Kind: reasoningKindRedacted,
				Data: rb.Data,
			})

		default:
			slog.Debug("anthropic: unhandled content_block_start type",
				"block_type", ev.ContentBlock.Type, "index", ev.Index)
		}

	case "content_block_delta":
		ev := event.AsContentBlockDelta()
		switch ev.Delta.Type {
		case "text_delta":
			text := ev.Delta.Text
			s.textContent.WriteString(text)
			return llmapi.Event{Type: llmapi.EventTextDelta, Text: text}, true

		case "thinking_delta":
			thinking := ev.Delta.Thinking
			s.reasoningContent.WriteString(thinking)
			if tb, ok := s.pendingThinking[ev.Index]; ok {
				tb.Text += thinking
			}
			return llmapi.Event{Type: llmapi.EventReasoningDelta, Reasoning: thinking}, true

		case "signature_delta":
			if tb, ok := s.pendingThinking[ev.Index]; ok {
				tb.Signature += ev.Delta.Signature
			}

		case "input_json_delta":
			if tc, ok := s.pendingCalls[ev.Index]; ok {
				tc.Function.Arguments += ev.Delta.PartialJSON
			}

		default:
			slog.Debug("anthropic: unhandled content_block_delta type",
				"delta_type", ev.Delta.Type, "index", ev.Index)
		}

	case "content_block_stop":
		ev := event.AsContentBlockStop()
		if tc, ok := s.pendingCalls[ev.Index]; ok {
			delete(s.pendingCalls, ev.Index)
			s.toolCalls = append(s.toolCalls, *tc)
			return llmapi.Event{
				Type:     llmapi.EventToolCallDone,
				ToolCall: tc,
			}, true
		}
		if tb, ok := s.pendingThinking[ev.Index]; ok {
			delete(s.pendingThinking, ev.Index)
			s.reasoningBlocks = append(s.reasoningBlocks, *tb)
		}
	}
	return llmapi.Event{}, false
}

func (s *streamIterator) buildDoneEvent() llmapi.Event {
	msg := llmapi.Message{
		Role:             llmapi.RoleAssistant,
		Content:          s.textContent.String(),
		ReasoningContent: s.reasoningContent.String(),
		ToolCalls:        s.toolCalls,
		ReasoningBlocks:  s.reasoningBlocks,
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

	usage := llmapi.Usage{
		TokensSent:         uncached + cached + cacheCreated,
		TokensReceived:     int(s.usage.OutputTokens) + int(s.usageDelta.OutputTokens),
		TokensCached:       cached,
		TokensCacheCreated: cacheCreated,
	}

	return llmapi.Event{
		Type: llmapi.EventStreamDone,
		DoneData: &llmapi.DoneData{
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
	s.reasoningBlocks = nil
	s.pendingThinking = nil
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

// mapStopReason converts Anthropic stop reasons to llmapi.FinishReason.
func mapStopReason(reason ant.StopReason) llmapi.FinishReason {
	switch reason {
	case ant.StopReasonEndTurn:
		return llmapi.FinishReasonStop
	case ant.StopReasonToolUse:
		return llmapi.FinishReasonToolCall
	case ant.StopReasonMaxTokens:
		return llmapi.FinishReasonLength
	case ant.StopReasonStopSequence:
		return llmapi.FinishReasonStop
	case ant.StopReasonPauseTurn:
		return llmapi.FinishReasonPause
	case ant.StopReasonRefusal:
		return llmapi.FinishReasonRefusal
	default:
		slog.Warn("anthropic: unmapped stop_reason", "reason", reason)
		return llmapi.FinishReasonNull
	}
}

// isRetryableMidStreamError reports whether err is a retryable error surfaced
// after the SSE stream has been established. The SDK reports SSE "error"
// events as *ant.Error with the underlying HTTP response (StatusCode 200,
// since the stream connection itself succeeded). Older releases formatted
// these as a plain "received error while streaming: ..." string, which
// ratelimit.IsRetryableStreamError still handles as a fallback.
func isRetryableMidStreamError(err error) bool {
	var apiErr *ant.Error
	if errors.As(err, &apiErr) && apiErr.Response != nil && apiErr.Response.StatusCode == http.StatusOK {
		switch apiErr.Type() {
		case ant.ErrorTypeOverloadedError,
			ant.ErrorTypeAPIError,
			ant.ErrorTypeRateLimitError:
			return true
		}
		return false
	}
	return ratelimit.IsRetryableStreamError(err)
}
