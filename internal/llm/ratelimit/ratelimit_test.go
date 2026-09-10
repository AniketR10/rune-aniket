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

package ratelimit

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsRetryableStreamError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "nil error",
			err:       nil,
			retryable: false,
		},
		{
			name:      "overloaded_error from Anthropic SSE",
			err:       fmt.Errorf(`received error while streaming: {"type":"error","error":{"details":null,"type":"overloaded_error","message":"Overloaded"},"request_id":"req_test123"}`),
			retryable: true,
		},
		{
			name:      "api_error from SSE",
			err:       fmt.Errorf(`received error while streaming: {"type":"error","error":{"type":"api_error","message":"Internal server error"}}`),
			retryable: true,
		},
		{
			name:      "rate_limit_error from SSE",
			err:       fmt.Errorf(`received error while streaming: {"type":"error","error":{"type":"rate_limit_error","message":"Rate limited"}}`),
			retryable: true,
		},
		{
			name:      "authentication_error is not retryable",
			err:       fmt.Errorf(`received error while streaming: {"type":"error","error":{"type":"authentication_error","message":"Invalid API key"}}`),
			retryable: false,
		},
		{
			name:      "invalid_request_error is not retryable",
			err:       fmt.Errorf(`received error while streaming: {"type":"error","error":{"type":"invalid_request_error","message":"Bad request"}}`),
			retryable: false,
		},
		{
			name:      "permission_error is not retryable",
			err:       fmt.Errorf(`received error while streaming: {"type":"error","error":{"type":"permission_error","message":"Forbidden"}}`),
			retryable: false,
		},
		{
			name:      "not_found_error is not retryable",
			err:       fmt.Errorf(`received error while streaming: {"type":"error","error":{"type":"not_found_error","message":"Not found"}}`),
			retryable: false,
		},
		{
			name:      "unrelated error is not retryable",
			err:       fmt.Errorf("invalid JSON response"),
			retryable: false,
		},
		{
			name:      "network error is not a stream error",
			err:       fmt.Errorf("connection reset by peer"),
			retryable: false,
		},
		{
			name:      "HTTP error with rate_limit_error in body is not a stream error",
			err:       fmt.Errorf(`429 Too Many Requests {"type":"rate_limit_error","message":"rate limited"}`),
			retryable: false,
		},
		{
			name:      "HTTP error with api_error in body is not a stream error",
			err:       fmt.Errorf(`500 Internal Server Error {"type":"api_error","message":"internal error"}`),
			retryable: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.retryable, IsRetryableStreamError(tt.err))
		})
	}
}

func TestRetryWait(t *testing.T) {
	base := time.Date(2026, 6, 7, 18, 0, 0, 0, time.UTC)
	prev := nowFunc
	nowFunc = func() time.Time { return base }
	t.Cleanup(func() { nowFunc = prev })

	t.Run("Retry-After integer wins", func(t *testing.T) {
		h := http.Header{}
		h.Set("Retry-After", "30")
		h.Set("anthropic-ratelimit-requests-reset", base.Add(5*time.Second).Format(time.RFC3339))
		assert.Equal(t, 30*time.Second, RetryWait(h, 0))
	})

	// Regression: a per-minute 429 omits Retry-After and carries reset
	// timestamps. Backoff must wait for the reset window, not fall back to
	// the 1s/2s/4s exponential default that gave up before the window
	// cleared.
	t.Run("reset timestamp drives backoff when Retry-After absent", func(t *testing.T) {
		h := http.Header{}
		h.Set("anthropic-ratelimit-input-tokens-reset", base.Add(42*time.Second).Format(time.RFC3339))
		assert.Equal(t, 42*time.Second, RetryWait(h, 0))
	})

	t.Run("soonest reset across resources is used", func(t *testing.T) {
		h := http.Header{}
		h.Set("anthropic-ratelimit-input-tokens-reset", base.Add(50*time.Second).Format(time.RFC3339))
		h.Set("anthropic-ratelimit-requests-reset", base.Add(12*time.Second).Format(time.RFC3339))
		assert.Equal(t, 12*time.Second, RetryWait(h, 0))
	})

	t.Run("reset wait is capped", func(t *testing.T) {
		h := http.Header{}
		h.Set("anthropic-ratelimit-requests-reset", base.Add(10*time.Minute).Format(time.RFC3339))
		assert.Equal(t, maxResetWait, RetryWait(h, 0))
	})

	t.Run("duration-form reset header", func(t *testing.T) {
		h := http.Header{}
		h.Set("anthropic-ratelimit-output-tokens-reset", "8s")
		assert.Equal(t, 8*time.Second, RetryWait(h, 0))
	})

	t.Run("falls back to exponential backoff without headers", func(t *testing.T) {
		// attempt 2 -> base 4s, plus up to 25% jitter.
		wait := RetryWait(http.Header{}, 2)
		assert.GreaterOrEqual(t, wait, 4*time.Second)
		assert.Less(t, wait, 5*time.Second)
	})
}
