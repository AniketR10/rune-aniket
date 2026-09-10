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
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	ant "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

func TestMapStopReason(t *testing.T) {
	tests := []struct {
		name string
		in   ant.StopReason
		want llmapi.FinishReason
		warn bool
	}{
		{"end_turn", ant.StopReasonEndTurn, llmapi.FinishReasonStop, false},
		{"tool_use", ant.StopReasonToolUse, llmapi.FinishReasonToolCall, false},
		{"max_tokens", ant.StopReasonMaxTokens, llmapi.FinishReasonLength, false},
		{"stop_sequence", ant.StopReasonStopSequence, llmapi.FinishReasonStop, false},
		{"pause_turn", ant.StopReasonPauseTurn, llmapi.FinishReasonPause, false},
		{"refusal", ant.StopReasonRefusal, llmapi.FinishReasonRefusal, false},
		{"unknown", ant.StopReason("frobnicate"), llmapi.FinishReasonNull, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
			defer slog.SetDefault(prev)

			got := mapStopReason(tt.in)
			assert.Equal(t, tt.want, got)

			warned := strings.Contains(buf.String(), "unmapped stop_reason")
			assert.Equal(t, tt.warn, warned, "warn log expectation mismatch: %q", buf.String())
		})
	}
}

// thinkingSSEResponse returns an SSE stream with a thinking block (text +
// signature deltas) followed by a visible text block and end_turn.
func thinkingSSEResponse() string {
	return `event: message_start
data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"let me "}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"think"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"sig-"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"abc=="}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"the answer"}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`
}

// redactedThinkingSSEResponse returns an SSE stream with a redacted_thinking
// block followed by a signed thinking block, exercising interleaving + order.
func redactedThinkingSSEResponse() string {
	return `event: message_start
data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"redacted_thinking","data":"encrypted-blob"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"thinking","thinking":"","signature":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"thinking_delta","thinking":"after redacted"}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"signature_delta","signature":"sig-2"}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":5}}

event: message_stop
data: {"type":"message_stop"}

`
}

func newTestClient(url string) *client {
	return &client{
		config: Config{MaxTokens: 1024},
		anthropic: ant.NewClient(
			option.WithAPIKey("test-key"),
			option.WithBaseURL(url),
			option.WithMaxRetries(0),
		),
	}
}

func collectDone(t *testing.T, it iterator.Iterator[llmapi.Event]) (*llmapi.DoneData, []llmapi.Event) {
	t.Helper()
	ctx := context.Background()
	var done *llmapi.DoneData
	var events []llmapi.Event
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		events = append(events, ev)
		if ev.Type == llmapi.EventStreamDone {
			done = ev.DoneData
		}
	}
	require.NoError(t, it.Err())
	_ = it.Close()
	return done, events
}

func TestStreamCapturesThinkingSignature(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(thinkingSSEResponse()))
	}))
	t.Cleanup(srv.Close)

	it, err := newTestClient(srv.URL).CreateCompletion(context.Background(),
		llmapi.ModelEntry{Name: "claude-test"},
		llmapi.Request{Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "hi"}}})
	require.NoError(t, err)

	done, _ := collectDone(t, it)
	require.NotNil(t, done)
	assert.Equal(t, "the answer", done.Message.Content)
	assert.Equal(t, "let me think", done.Message.ReasoningContent)
	require.Len(t, done.Message.ReasoningBlocks, 1)
	assert.Equal(t, llmapi.ReasoningBlock{
		Kind:      "thinking",
		Text:      "let me think",
		Signature: "sig-abc==",
	}, done.Message.ReasoningBlocks[0])
}

func TestStreamCapturesRedactedThinking(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(redactedThinkingSSEResponse()))
	}))
	t.Cleanup(srv.Close)

	it, err := newTestClient(srv.URL).CreateCompletion(context.Background(),
		llmapi.ModelEntry{Name: "claude-test"},
		llmapi.Request{Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "hi"}}})
	require.NoError(t, err)

	done, _ := collectDone(t, it)
	require.NotNil(t, done)
	require.Len(t, done.Message.ReasoningBlocks, 2)
	assert.Equal(t, llmapi.ReasoningBlock{Kind: "redacted", Data: "encrypted-blob"},
		done.Message.ReasoningBlocks[0])
	assert.Equal(t, llmapi.ReasoningBlock{Kind: "thinking", Text: "after redacted", Signature: "sig-2"},
		done.Message.ReasoningBlocks[1])
}

func TestStreamResetClearsReasoningBlocks(t *testing.T) {
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := attempts.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if n == 1 {
			// Emit a thinking block, then fail mid-stream so the iterator
			// retries and must discard the partially accumulated reasoning.
			_, _ = w.Write([]byte(partialSSEWithThinkingThenOverload()))
			return
		}
		_, _ = w.Write([]byte(thinkingSSEResponse()))
	}))
	t.Cleanup(srv.Close)

	it, err := newTestClient(srv.URL).CreateCompletion(context.Background(),
		llmapi.ModelEntry{Name: "claude-test"},
		llmapi.Request{Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "hi"}}})
	require.NoError(t, err)

	done, events := collectDone(t, it)
	require.NotNil(t, done)

	var gotReset bool
	for _, ev := range events {
		if ev.Type == llmapi.EventStreamReset {
			gotReset = true
		}
	}
	assert.True(t, gotReset, "expected a stream reset before retry")
	// After reset the second stream's single thinking block must be the only
	// one present — no duplicate from the discarded first attempt.
	require.Len(t, done.Message.ReasoningBlocks, 1)
	assert.Equal(t, "sig-abc==", done.Message.ReasoningBlocks[0].Signature)
}

// partialSSEWithThinkingThenOverload emits a complete thinking block and then
// an overloaded_error, forcing a retryable mid-stream failure.
func partialSSEWithThinkingThenOverload() string {
	return `event: message_start
data: {"type":"message_start","message":{"id":"msg_test","type":"message","role":"assistant","content":[],"model":"test","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":10,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"discarded"}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"signature_delta","signature":"discarded-sig"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: error
data: {"type":"error","error":{"details":null,"type":"overloaded_error","message":"Overloaded"},"request_id":"req_test123"}

`
}
