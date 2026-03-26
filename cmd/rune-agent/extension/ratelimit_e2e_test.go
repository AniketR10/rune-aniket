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

package extension

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguetui"
	"unstable.build/go-tui/cmd/rune-agent/llm"
	"unstable.build/go-tui/cmd/rune-agent/llm/openai"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// minimalSSEResponse returns a valid SSE response that the OpenAI SDK can parse.
func minimalSSEResponse() string {
	return "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"\"}, \"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"
}

// TestRateLimitE2E_429RetryWarningRenderedInTUI is an end-to-end integration
// test that verifies the full pipeline from an HTTP 429 response through to
// the dialoguetui.Component rendering the rate limit warning.
//
// Pipeline: httptest.Server (429→200) → openai.Client → llm.Event iterator
// → collect EventRateLimitWarning → feed as MessageEventWarning → Handler
// → Component.AddWarningMessage → verify rendered output.
func TestRateLimitE2E_429RetryWarningRenderedInTUI(t *testing.T) {
	// Step 1: Mock HTTP server that returns 429 once, then succeeds.
	var attempts atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.Header().Set("Retry-After", "0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"rate limited"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(minimalSSEResponse()))
	}))
	t.Cleanup(srv.Close)

	// Step 2: Create OpenAI client pointing at mock server.
	c := openai.NewClient("test-key", openai.Config{
		Model:   openai.GPT3Dot5Turbo,
		BaseURL: srv.URL,
	}, openai.AvailableModels())

	// Step 3: Call CreateCompletion and collect all events.
	ctx := context.Background()
	req := llm.Request{Messages: []llm.Message{
		{Role: llm.RoleUser, Content: "hello"},
	}}
	it, err := c.CreateCompletion(ctx, req)
	require.NoError(t, err)

	var warnings []llm.Event
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		if ev.Type == llm.EventRateLimitWarning {
			warnings = append(warnings, ev)
		}
	}
	require.NoError(t, it.Err())
	_ = it.Close()

	// Verify the client produced at least one warning.
	require.NotEmpty(t, warnings, "expected at least one rate limit warning from the client")
	require.NotNil(t, warnings[0].RateLimit)
	assert.NotEmpty(t, warnings[0].RateLimit.Message)

	// Step 4: Feed warnings into a dialoguetui.Handler and verify rendering.
	interrupt := make(chan struct{}, 10)
	h, tx, _ := dialoguetui.Handler(ctx, new(sync.Mutex),
		dialoguetui.NewComponent(dialoguetui.ComponentConfig{}),
		term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}))
	defer close(tx)
	h.Resize(60, 9)

	for _, w := range warnings {
		tx <- dialoguetui.MessageEvent{
			Type: dialoguetui.MessageEventWarning,
			Text: w.RateLimit.Message,
		}
		<-interrupt
	}

	w := term.NewStringWriter(60, 9)
	h.Draw(w)
	err = w.Flush()
	require.NoError(t, err)

	out := w.String()
	assert.Contains(t, out, "Retrying")
	assert.Contains(t, out, "attempt 1/3")
	// Warnings should NOT have the "! " prefix that errors have.
	assert.NotContains(t, out, "! Retrying")
}

// TestRateLimitE2E_AnthropicProactiveWarningRenderedInTUI tests the proactive
// Anthropic rate limit header warning path: when a successful response arrives
// with <10% remaining tokens, a warning is emitted and rendered in the TUI.
//
// Pipeline: httptest.Server (200 with low-remaining headers) → openai.Client
// → llm.Event iterator → EventRateLimitWarning → MessageEventWarning
// → Handler → Component.AddWarningMessage → verify rendered output.
func TestRateLimitE2E_AnthropicProactiveWarningRenderedInTUI(t *testing.T) {
	// Step 1: Mock HTTP server that succeeds but includes low-remaining headers.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Signal that input tokens are almost exhausted.
		w.Header().Set("anthropic-ratelimit-input-tokens-limit", "30000")
		w.Header().Set("anthropic-ratelimit-input-tokens-remaining", "1500")
		w.Header().Set("anthropic-ratelimit-input-tokens-reset", "2026-03-11T12:00:00Z")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(minimalSSEResponse()))
	}))
	t.Cleanup(srv.Close)

	// Step 2: Create OpenAI client with an Anthropic-like URL.
	c := openai.NewClient("test-key", openai.Config{
		Model:   openai.GPT3Dot5Turbo,
		BaseURL: srv.URL + "/v1/anthropic.com/",
	}, openai.AvailableModels())

	// Step 3: Consume the stream and collect warnings.
	ctx := context.Background()
	req := llm.Request{Messages: []llm.Message{
		{Role: llm.RoleUser, Content: "hello"},
	}}
	it, err := c.CreateCompletion(ctx, req)
	require.NoError(t, err)

	var warnings []llm.Event
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		if ev.Type == llm.EventRateLimitWarning {
			warnings = append(warnings, ev)
		}
	}
	require.NoError(t, it.Err())
	_ = it.Close()

	// Verify proactive Anthropic warning.
	require.Len(t, warnings, 1, "expected exactly one proactive Anthropic rate limit warning")
	require.NotNil(t, warnings[0].RateLimit)
	assert.Contains(t, warnings[0].RateLimit.Message, "1500")
	assert.Contains(t, warnings[0].RateLimit.Message, "30000")
	assert.Contains(t, warnings[0].RateLimit.Message, "console.anthropic.com")

	// Step 4: Feed warning into a dialoguetui.Handler and verify rendering.
	interrupt := make(chan struct{}, 10)
	h, tx, _ := dialoguetui.Handler(ctx, new(sync.Mutex),
		dialoguetui.NewComponent(dialoguetui.ComponentConfig{}),
		term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}))
	defer close(tx)
	h.Resize(80, 9)

	tx <- dialoguetui.MessageEvent{
		Type: dialoguetui.MessageEventWarning,
		Text: warnings[0].RateLimit.Message,
	}
	<-interrupt

	w := term.NewStringWriter(80, 9)
	h.Draw(w)
	err = w.Flush()
	require.NoError(t, err)

	out := w.String()
	assert.Contains(t, out, "Approaching Anthropic rate limit")
	assert.Contains(t, out, "1500")
	assert.Contains(t, out, "30000")
	assert.Contains(t, out, "console.anthropic.com")
}

// TestRateLimitE2E_GenericProviderWarningRenderedInTUI tests that non-Anthropic
// providers also get a proactive warning when standard x-ratelimit-* headers
// indicate resources are running low.
func TestRateLimitE2E_GenericProviderWarningRenderedInTUI(t *testing.T) {
	// Step 1: Mock HTTP server that succeeds but with low remaining requests.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-ratelimit-limit-requests", "500")
		w.Header().Set("x-ratelimit-remaining-requests", "20")
		w.Header().Set("x-ratelimit-reset-requests", "30s")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(minimalSSEResponse()))
	}))
	t.Cleanup(srv.Close)

	// Step 2: Non-Anthropic URL.
	c := openai.NewClient("test-key", openai.Config{
		Model:   openai.GPT3Dot5Turbo,
		BaseURL: srv.URL,
	}, openai.AvailableModels())

	// Step 3: Consume stream and collect warnings.
	ctx := context.Background()
	req := llm.Request{Messages: []llm.Message{
		{Role: llm.RoleUser, Content: "hello"},
	}}
	it, err := c.CreateCompletion(ctx, req)
	require.NoError(t, err)

	var warnings []llm.Event
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		if ev.Type == llm.EventRateLimitWarning {
			warnings = append(warnings, ev)
		}
	}
	require.NoError(t, it.Err())
	_ = it.Close()

	// Verify generic warning.
	require.Len(t, warnings, 1, "expected one generic rate limit warning")
	require.NotNil(t, warnings[0].RateLimit)
	assert.Contains(t, warnings[0].RateLimit.Message, "20")
	assert.Contains(t, warnings[0].RateLimit.Message, "500")
	assert.Contains(t, warnings[0].RateLimit.Message, "requests")

	// Step 4: Feed into TUI and verify rendering.
	interrupt := make(chan struct{}, 10)
	h, tx, _ := dialoguetui.Handler(ctx, new(sync.Mutex),
		dialoguetui.NewComponent(dialoguetui.ComponentConfig{}),
		term.FuncInterrupter(func(context.Context) error {
			interrupt <- struct{}{}
			return nil
		}))
	defer close(tx)
	h.Resize(60, 9)

	tx <- dialoguetui.MessageEvent{
		Type: dialoguetui.MessageEventWarning,
		Text: warnings[0].RateLimit.Message,
	}
	<-interrupt

	w := term.NewStringWriter(60, 9)
	h.Draw(w)
	err = w.Flush()
	require.NoError(t, err)

	out := w.String()
	assert.Contains(t, out, "Approaching rate limit")
	assert.Contains(t, out, "20")
	assert.Contains(t, out, "500")
}
