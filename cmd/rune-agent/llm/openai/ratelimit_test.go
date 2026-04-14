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

package openai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/rune-agent/llm"
)

// minimalSSEResponse returns a valid SSE response that the OpenAI SDK can parse.
func minimalSSEResponse() string {
	return "data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"\"}, \"finish_reason\":\"stop\"}]}\n\n" +
		"data: [DONE]\n\n"
}

func TestRateLimitRetry(t *testing.T) {
	t.Run("retries on 429 with retry-after header", func(t *testing.T) {
		var attempts atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := attempts.Add(1)
			if n <= 2 {
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

		c := NewClient("test-key", Config{
			Model:   GPT3Dot5Turbo,
			BaseURL: srv.URL,
		}, AvailableModels())

		ctx := context.Background()
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		}}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)

		var warnings []llm.Event
		var textChunks []string
		var gotDone bool
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			switch ev.Type {
			case llm.EventRateLimitWarning:
				warnings = append(warnings, ev)
			case llm.EventTextDelta:
				textChunks = append(textChunks, ev.Text)
			case llm.EventStreamDone:
				gotDone = true
			}
		}
		require.NoError(t, it.Err())
		_ = it.Close()

		assert.Equal(t, int32(3), attempts.Load(), "should have made 3 attempts")
		assert.Len(t, warnings, 2, "should have 2 rate limit warnings (one per retry)")
		for _, w := range warnings {
			require.NotNil(t, w.RateLimit)
			assert.NotEmpty(t, w.RateLimit.Message)
		}
		assert.True(t, gotDone, "should have received done event")
		assert.Contains(t, textChunks, "hello")
	})

	t.Run("does not retry insufficient_quota 429", func(t *testing.T) {
		var attempts atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"type":"insufficient_quota","message":"You exceeded your current quota, please check your plan and billing details.","code":"insufficient_quota"}}`))
		}))
		t.Cleanup(srv.Close)

		c := NewClient("test-key", Config{
			Model:   GPT3Dot5Turbo,
			BaseURL: srv.URL,
		}, AvailableModels())

		ctx := context.Background()
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		}}
		_, err := c.CreateCompletion(ctx, req)
		require.Error(t, err)
		assert.Equal(t, int32(1), attempts.Load(), "should not retry insufficient_quota")
	})

	t.Run("does not retry non-retryable errors", func(t *testing.T) {
		var attempts atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"bad request"}}`))
		}))
		t.Cleanup(srv.Close)

		c := NewClient("test-key", Config{
			Model:   GPT3Dot5Turbo,
			BaseURL: srv.URL,
		}, AvailableModels())

		ctx := context.Background()
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		}}
		_, err := c.CreateCompletion(ctx, req)
		require.Error(t, err)
		assert.Equal(t, int32(1), attempts.Load(), "should not retry 400 errors")
	})

	t.Run("stops retrying after max attempts", func(t *testing.T) {
		var attempts atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts.Add(1)
			w.Header().Set("Retry-After", "0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"rate limited"}}`))
		}))
		t.Cleanup(srv.Close)

		c := NewClient("test-key", Config{
			Model:   GPT3Dot5Turbo,
			BaseURL: srv.URL,
		}, AvailableModels())

		ctx := context.Background()
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		}}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)

		var gotError bool
		var warnings int
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			switch ev.Type {
			case llm.EventRateLimitWarning:
				warnings++
			case llm.EventStreamError:
				gotError = true
			}
		}

		assert.Equal(t, int32(maxStreamRetries+1), attempts.Load(),
			"should make initial + maxStreamRetries attempts")
		assert.Equal(t, maxStreamRetries, warnings)
		assert.True(t, gotError, "should surface error after exhausting retries")
	})
}

func TestToolCallParseRetry(t *testing.T) {
	ollamaErrorBody := `{"error":{"type":"api_error","message":"error parsing tool call: raw='text{\"cmd\":\"x\"}', err=invalid character 'W'"}}`

	t.Run("retries and succeeds", func(t *testing.T) {
		var attempts atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := attempts.Add(1)
			if n <= 1 {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(ollamaErrorBody))
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(minimalSSEResponse()))
		}))
		t.Cleanup(srv.Close)

		c := NewClient("test-key", Config{
			Model:   GPT3Dot5Turbo,
			BaseURL: srv.URL,
		}, AvailableModels())

		ctx := context.Background()
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		}}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)

		var gotDone bool
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			if ev.Type == llm.EventStreamDone {
				gotDone = true
			}
		}
		require.NoError(t, it.Err())
		_ = it.Close()

		assert.Equal(t, int32(2), attempts.Load(), "should retry once then succeed")
		assert.True(t, gotDone)
	})

	t.Run("exhausted retries returns ToolCallParseError", func(t *testing.T) {
		var attempts atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts.Add(1)
			w.Header().Set("Retry-After", "0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(ollamaErrorBody))
		}))
		t.Cleanup(srv.Close)

		c := NewClient("test-key", Config{
			Model:   GPT3Dot5Turbo,
			BaseURL: srv.URL,
		}, AvailableModels())

		ctx := context.Background()
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		}}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)

		var streamErr error
		var warnings int
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			switch ev.Type {
			case llm.EventRateLimitWarning:
				warnings++
			case llm.EventStreamError:
				streamErr = ev.Error
			}
		}

		require.Error(t, streamErr)
		var tcErr *ToolCallParseError
		assert.ErrorAs(t, streamErr, &tcErr)
		assert.Equal(t, maxStreamRetries, warnings,
			"should emit retry warnings")
		assert.Equal(t, int32(maxStreamRetries+1), attempts.Load(),
			"should exhaust all retries")
	})
}

func TestRateLimitHeaderCheck(t *testing.T) {
	t.Run("emits warning when Anthropic input tokens below 10 percent", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("anthropic-ratelimit-input-tokens-limit", "30000")
			w.Header().Set("anthropic-ratelimit-input-tokens-remaining", "2000")
			w.Header().Set("anthropic-ratelimit-input-tokens-reset", "2026-03-11T12:00:00Z")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(minimalSSEResponse()))
		}))
		t.Cleanup(srv.Close)

		c := NewClient("test-key", Config{
			Model:   GPT3Dot5Turbo,
			BaseURL: srv.URL + "/v1/anthropic.com/",
		}, AvailableModels())

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
		_ = it.Close()

		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0].RateLimit.Message, "2000")
		assert.Contains(t, warnings[0].RateLimit.Message, "30000")
	})

	t.Run("no warning when Anthropic tokens well above threshold", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("anthropic-ratelimit-input-tokens-limit", "30000")
			w.Header().Set("anthropic-ratelimit-input-tokens-remaining", "25000")
			w.Header().Set("anthropic-ratelimit-output-tokens-limit", "10000")
			w.Header().Set("anthropic-ratelimit-output-tokens-remaining", "9000")
			w.Header().Set("anthropic-ratelimit-requests-limit", "100")
			w.Header().Set("anthropic-ratelimit-requests-remaining", "90")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(minimalSSEResponse()))
		}))
		t.Cleanup(srv.Close)

		c := NewClient("test-key", Config{
			Model:   GPT3Dot5Turbo,
			BaseURL: srv.URL + "/v1/anthropic.com/",
		}, AvailableModels())

		ctx := context.Background()
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		}}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)

		var warnings int
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			if ev.Type == llm.EventRateLimitWarning {
				warnings++
			}
		}
		_ = it.Close()

		assert.Equal(t, 0, warnings, "should not warn when well above threshold")
	})

	t.Run("no warning for non-Anthropic URLs", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			// Even with low remaining, non-Anthropic URLs should not trigger warning
			w.Header().Set("anthropic-ratelimit-input-tokens-limit", "30000")
			w.Header().Set("anthropic-ratelimit-input-tokens-remaining", "100")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(minimalSSEResponse()))
		}))
		t.Cleanup(srv.Close)

		c := NewClient("test-key", Config{
			Model:   GPT3Dot5Turbo,
			BaseURL: srv.URL,
		}, AvailableModels())

		ctx := context.Background()
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		}}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)

		var warnings int
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			if ev.Type == llm.EventRateLimitWarning {
				warnings++
			}
		}
		_ = it.Close()

		assert.Equal(t, 0, warnings)
	})

	t.Run("emits warning for output tokens below threshold", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("anthropic-ratelimit-input-tokens-limit", "30000")
			w.Header().Set("anthropic-ratelimit-input-tokens-remaining", "25000")
			w.Header().Set("anthropic-ratelimit-output-tokens-limit", "10000")
			w.Header().Set("anthropic-ratelimit-output-tokens-remaining", "500")
			w.Header().Set("anthropic-ratelimit-output-tokens-reset", "2026-03-11T12:00:00Z")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(minimalSSEResponse()))
		}))
		t.Cleanup(srv.Close)

		c := NewClient("test-key", Config{
			Model:   GPT3Dot5Turbo,
			BaseURL: srv.URL + "/v1/anthropic.com/",
		}, AvailableModels())

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
		_ = it.Close()

		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0].RateLimit.Message, "output")
	})
}

func TestStandardRateLimitHeaderCheck(t *testing.T) {
	t.Run("emits warning when requests below threshold", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("x-ratelimit-limit-requests", "100")
			w.Header().Set("x-ratelimit-remaining-requests", "5")
			w.Header().Set("x-ratelimit-reset-requests", "1s")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(minimalSSEResponse()))
		}))
		t.Cleanup(srv.Close)

		// Non-Anthropic URL — should use standard headers.
		c := NewClient("test-key", Config{
			Model:   GPT3Dot5Turbo,
			BaseURL: srv.URL,
		}, AvailableModels())

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
		_ = it.Close()

		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0].RateLimit.Message, "5")
		assert.Contains(t, warnings[0].RateLimit.Message, "100")
		assert.Contains(t, warnings[0].RateLimit.Message, "requests")
	})

	t.Run("emits warning when tokens below threshold", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("x-ratelimit-limit-tokens", "200000")
			w.Header().Set("x-ratelimit-remaining-tokens", "10000")
			w.Header().Set("x-ratelimit-reset-tokens", "6s")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(minimalSSEResponse()))
		}))
		t.Cleanup(srv.Close)

		c := NewClient("test-key", Config{
			Model:   GPT3Dot5Turbo,
			BaseURL: srv.URL,
		}, AvailableModels())

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
		_ = it.Close()

		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0].RateLimit.Message, "10000")
		assert.Contains(t, warnings[0].RateLimit.Message, "200000")
		assert.Contains(t, warnings[0].RateLimit.Message, "tokens")
	})

	t.Run("no warning when well above threshold", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("x-ratelimit-limit-requests", "100")
			w.Header().Set("x-ratelimit-remaining-requests", "90")
			w.Header().Set("x-ratelimit-limit-tokens", "200000")
			w.Header().Set("x-ratelimit-remaining-tokens", "180000")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(minimalSSEResponse()))
		}))
		t.Cleanup(srv.Close)

		c := NewClient("test-key", Config{
			Model:   GPT3Dot5Turbo,
			BaseURL: srv.URL,
		}, AvailableModels())

		ctx := context.Background()
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		}}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)

		var warnings int
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			if ev.Type == llm.EventRateLimitWarning {
				warnings++
			}
		}
		_ = it.Close()

		assert.Equal(t, 0, warnings)
	})
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		retryable bool
	}{
		// Retryable status codes (matching official OpenAI SDK).
		{"408 Request Timeout", http.StatusRequestTimeout, true},
		{"409 Conflict", http.StatusConflict, true},
		{"429 Too Many Requests", http.StatusTooManyRequests, true},
		{"500 Internal Server Error", http.StatusInternalServerError, true},
		{"502 Bad Gateway", http.StatusBadGateway, true},
		{"503 Service Unavailable", http.StatusServiceUnavailable, true},
		{"504 Gateway Timeout", http.StatusGatewayTimeout, true},
		{"529 Overloaded (Anthropic)", 529, true},
		// Non-retryable status codes.
		{"400 Bad Request", http.StatusBadRequest, false},
		{"401 Unauthorized", http.StatusUnauthorized, false},
		{"403 Forbidden", http.StatusForbidden, false},
		{"404 Not Found", http.StatusNotFound, false},
		{"413 Request Too Large", http.StatusRequestEntityTooLarge, false},
		{"422 Unprocessable Entity", http.StatusUnprocessableEntity, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("status_%d", tt.status), func(t *testing.T) {
			err := &openaiError{statusCode: tt.status}
			assert.Equal(t, tt.retryable, isRetryableError(err), tt.name)
		})
	}

	t.Run("non-HTTP error is not retryable", func(t *testing.T) {
		err := fmt.Errorf("network error")
		assert.False(t, isRetryableError(err))
	})

	t.Run("429 with insufficient_quota is not retryable", func(t *testing.T) {
		err := &openaiError{
			statusCode: http.StatusTooManyRequests,
			code:       "insufficient_quota",
			message:    "You exceeded your current quota",
		}
		assert.False(t, isRetryableError(err))
	})

	t.Run("429 with rate_limit_exceeded is retryable", func(t *testing.T) {
		err := &openaiError{
			statusCode: http.StatusTooManyRequests,
			code:       "rate_limit_exceeded",
		}
		assert.True(t, isRetryableError(err))
	})

	t.Run("429 with empty code is retryable", func(t *testing.T) {
		err := &openaiError{
			statusCode: http.StatusTooManyRequests,
		}
		assert.True(t, isRetryableError(err))
	})

	t.Run("x-should-retry true overrides non-retryable status", func(t *testing.T) {
		yes := true
		err := &openaiError{
			statusCode:  http.StatusBadRequest,
			shouldRetry: &yes,
		}
		assert.True(t, isRetryableError(err))
	})

	t.Run("x-should-retry false overrides retryable status", func(t *testing.T) {
		no := false
		err := &openaiError{
			statusCode:  http.StatusTooManyRequests,
			shouldRetry: &no,
		}
		assert.False(t, isRetryableError(err))
	})

	t.Run("x-should-retry false overrides retryable 500", func(t *testing.T) {
		no := false
		err := &openaiError{
			statusCode:  http.StatusInternalServerError,
			shouldRetry: &no,
		}
		assert.False(t, isRetryableError(err))
	})

	t.Run("x-should-retry absent falls through to normal logic", func(t *testing.T) {
		err := &openaiError{
			statusCode:  http.StatusBadRequest,
			shouldRetry: nil,
		}
		assert.False(t, isRetryableError(err))
	})
}

// openaiError simulates the SDK's error type for testing isRetryableError.
// It implements the statusCoder, apiErrorCoder, and shouldRetryer interfaces.
type openaiError struct {
	statusCode  int
	message     string
	code        string
	shouldRetry *bool // nil = header absent, non-nil = header present
}

func (e *openaiError) Error() string {
	if e.message != "" {
		return e.message
	}
	return fmt.Sprintf("status %d", e.statusCode)
}

func (e *openaiError) StatusCode() int {
	return e.statusCode
}

func (e *openaiError) APIErrorCode() string {
	return e.code
}

func (e *openaiError) ShouldRetry() (bool, bool) {
	if e.shouldRetry == nil {
		return false, false
	}
	return *e.shouldRetry, true
}

func TestIsTransientNetworkError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		transient bool
	}{
		{
			name:      "connection reset by peer",
			err:       fmt.Errorf("read tcp 10.0.0.1:1234->172.66.0.1:443: read: connection reset by peer"),
			transient: true,
		},
		{
			name:      "broken pipe",
			err:       fmt.Errorf("write: broken pipe"),
			transient: true,
		},
		{
			name:      "connection refused",
			err:       fmt.Errorf("dial tcp 127.0.0.1:8080: connection refused"),
			transient: true,
		},
		{
			name:      "i/o timeout",
			err:       fmt.Errorf("read tcp: i/o timeout"),
			transient: true,
		},
		{
			name:      "TLS handshake timeout",
			err:       fmt.Errorf("net/http: TLS handshake timeout"),
			transient: true,
		},
		{
			name:      "server closed idle connection",
			err:       fmt.Errorf("http: server closed idle connection"),
			transient: true,
		},
		{
			name:      "use of closed network connection",
			err:       fmt.Errorf("use of closed network connection"),
			transient: true,
		},
		{
			name:      "wrapped connection reset",
			err:       fmt.Errorf("stream error: %w", fmt.Errorf("connection reset by peer")),
			transient: true,
		},
		{
			name:      "non-network error",
			err:       fmt.Errorf("invalid JSON response"),
			transient: false,
		},
		{
			name:      "nil error",
			err:       nil,
			transient: false,
		},
		{
			name:      "timeout net.Error",
			err:       &timeoutError{},
			transient: true,
		},
		{
			name:      "unexpected EOF",
			err:       fmt.Errorf("stream read: %w", io.ErrUnexpectedEOF),
			transient: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.transient, isTransientNetworkError(tt.err))
		})
	}
}

// timeoutError implements net.Error with Timeout() returning true.
type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return true }

func TestIsRetryableError_NetworkErrors(t *testing.T) {
	t.Run("connection reset is retryable", func(t *testing.T) {
		err := fmt.Errorf("read tcp: connection reset by peer")
		assert.True(t, isRetryableError(err))
	})

	t.Run("non-network non-HTTP error is not retryable", func(t *testing.T) {
		err := fmt.Errorf("invalid JSON")
		assert.False(t, isRetryableError(err))
	})
}

func TestRetryNetworkMessage(t *testing.T) {
	err := fmt.Errorf("connection reset by peer")
	msg := retryNetworkMessage(err, 2*time.Second, 1, 3)
	assert.Contains(t, msg, "Connection lost")
	assert.Contains(t, msg, "connection reset by peer")
	assert.Contains(t, msg, "2s")
	assert.Contains(t, msg, "1/3")
}

func TestMidStreamRetry(t *testing.T) {
	t.Run("retries on connection drop mid-stream then succeeds", func(t *testing.T) {
		var attempts atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := attempts.Add(1)
			if n == 1 {
				// First request: send partial data then close connection.
				w.Header().Set("Content-Type", "text/event-stream")
				w.WriteHeader(http.StatusOK)
				flusher, _ := w.(http.Flusher)
				_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"))
				flusher.Flush()
				// Close the connection abruptly by hijacking.
				hj, ok := w.(http.Hijacker)
				if !ok {
					t.Fatal("server does not support hijacking")
				}
				conn, _, _ := hj.Hijack()
				_ = conn.Close()
				return
			}
			// Second request: complete response.
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(minimalSSEResponse()))
		}))
		t.Cleanup(srv.Close)

		c := NewClient("test-key", Config{
			Model:   GPT3Dot5Turbo,
			BaseURL: srv.URL,
		}, AvailableModels())

		ctx := context.Background()
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		}}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)

		var events []llm.Event
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			events = append(events, ev)
		}
		require.NoError(t, it.Err())
		_ = it.Close()

		assert.Equal(t, int32(2), attempts.Load(), "should have made 2 attempts")

		// Should have a reset event followed by a warning.
		var gotReset, gotWarning, gotDone bool
		var textChunks []string
		for _, ev := range events {
			switch ev.Type {
			case llm.EventStreamReset:
				gotReset = true
			case llm.EventRateLimitWarning:
				gotWarning = true
				assert.Contains(t, ev.RateLimit.Message, "Connection lost")
			case llm.EventTextDelta:
				textChunks = append(textChunks, ev.Text)
			case llm.EventStreamDone:
				gotDone = true
			}
		}
		assert.True(t, gotReset, "should have received stream reset event")
		assert.True(t, gotWarning, "should have received rate limit warning")
		assert.True(t, gotDone, "should have received done event")
		assert.Contains(t, textChunks, "hello", "should have text from successful stream")
	})

	t.Run("exhausts mid-stream retries and surfaces error", func(t *testing.T) {
		var attempts atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts.Add(1)
			// Always send partial data then drop connection.
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher, _ := w.(http.Flusher)
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n\n"))
			flusher.Flush()
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("server does not support hijacking")
			}
			conn, _, _ := hj.Hijack()
			_ = conn.Close()
		}))
		t.Cleanup(srv.Close)

		c := NewClient("test-key", Config{
			Model:   GPT3Dot5Turbo,
			BaseURL: srv.URL,
		}, AvailableModels())

		ctx := context.Background()
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		}}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)

		var resets, warnings int
		var gotError bool
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			switch ev.Type {
			case llm.EventStreamReset:
				resets++
			case llm.EventRateLimitWarning:
				warnings++
			case llm.EventStreamError:
				gotError = true
			}
		}

		// 1 initial + maxMidStreamRetries retries = 4 total attempts
		assert.Equal(t, int32(maxMidStreamRetries+1), attempts.Load(),
			"should exhaust all mid-stream retries")
		assert.Equal(t, maxMidStreamRetries, resets,
			"should emit one reset per retry")
		assert.Equal(t, maxMidStreamRetries, warnings,
			"should emit one warning per retry")
		assert.True(t, gotError, "should surface error after exhausting retries")
	})
}

func TestRetryWait(t *testing.T) {
	t.Run("uses retry-after header when present", func(t *testing.T) {
		h := make(http.Header)
		h.Set("Retry-After", "5")
		d := retryWait(h, 0)
		assert.Equal(t, 5*1_000_000_000, int(d)) // 5 seconds in nanoseconds
	})

	t.Run("uses exponential backoff with jitter when no retry-after", func(t *testing.T) {
		d0 := retryWait(nil, 0)
		d1 := retryWait(nil, 1)
		d2 := retryWait(nil, 2)
		// Base values: 1s, 2s, 4s + up to 25% jitter.
		assert.True(t, d0 >= 1*time.Second, "attempt 0 should wait at least 1s")
		assert.True(t, d0 <= 1*time.Second+250*time.Millisecond, "attempt 0 jitter should be at most 250ms")
		assert.True(t, d1 >= 2*time.Second, "attempt 1 should wait at least 2s")
		assert.True(t, d2 >= 4*time.Second, "attempt 2 should wait at least 4s")
	})
}
