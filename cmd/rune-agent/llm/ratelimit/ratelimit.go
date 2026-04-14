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

// Package ratelimit provides shared retry and rate-limit utilities used by
// multiple LLM provider clients.
package ratelimit

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/unstablebuild/rune-go-sdk/retry"
	"unstable.build/go-tui/cmd/rune-agent/llm"
)

// MaxStreamRetries is the maximum number of retry attempts for retryable
// errors (408, 409, 429, 5xx) when creating a completion stream.
const MaxStreamRetries = 3

// MaxMidStreamRetries is the maximum number of retry attempts when a
// transient network error occurs after the stream has already started.
const MaxMidStreamRetries = 3

// WarningThreshold is the fraction of remaining capacity below
// which a proactive rate-limit warning is emitted. 0.1 means warn when <10% remains.
const WarningThreshold = 0.1

// retryableStreamErrorTypes are SSE error event type values that indicate
// transient server-side failures worth retrying. These match the "type" field
// inside the error object of SSE error events (e.g. overloaded_error from
// Anthropic). Non-retryable types (authentication_error, invalid_request_error,
// permission_error, not_found_error) are intentionally excluded.
var retryableStreamErrorTypes = []string{
	`"type":"overloaded_error"`,
	`"type":"api_error"`,
	`"type":"rate_limit_error"`,
}

// sseErrorPrefix is the prefix the SDK adds when an SSE "error" event is
// received during streaming. This distinguishes SSE stream errors from
// HTTP-level API errors whose messages may contain similar JSON.
const sseErrorPrefix = "received error while streaming"

// IsRetryableStreamError reports whether err represents a retryable SSE
// streaming error event from the API (e.g. overloaded_error, api_error,
// rate_limit_error). These errors arrive as SSE "error" events after the
// HTTP connection has been established, so no Retry-After header is available.
func IsRetryableStreamError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if !strings.Contains(msg, sseErrorPrefix) {
		return false
	}
	for _, s := range retryableStreamErrorTypes {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// RetryStreamMessage composes a human-readable retry message for SSE stream errors.
func RetryStreamMessage(err error, wait time.Duration, attempt, maxAttempts int) string {
	return fmt.Sprintf("Stream error (%s), retrying in %s (attempt %d/%d).",
		err, wait.Truncate(time.Second), attempt, maxAttempts)
}

// TransientNetworkSubstrings are error message fragments that indicate
// transient network failures worth retrying.
var transientNetworkSubstrings = []string{
	"connection reset by peer",
	"broken pipe",
	"connection refused",
	"i/o timeout",
	"TLS handshake timeout",
	"server closed idle connection",
	"use of closed network connection",
	"unexpected EOF",
}

// IsTransientNetworkError reports whether err is a transient network error
// that may succeed on retry.
func IsTransientNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	msg := err.Error()
	for _, s := range transientNetworkSubstrings {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// RetryWait determines how long to wait before the next retry.
// It prefers the Retry-After header; otherwise falls back to exponential backoff.
func RetryWait(headers http.Header, attempt int) time.Duration {
	if headers != nil {
		if ra := headers.Get("Retry-After"); ra != "" {
			if secs, err := strconv.Atoi(ra); err == nil && secs >= 0 {
				return time.Duration(secs) * time.Second
			}
		}
	}
	d := time.Duration(1<<uint(attempt)) * time.Second
	d = min(d, 60*time.Second)
	jitter := time.Duration(rand.Int64N(int64(d) / 4))
	return d + jitter
}

// RetryMessage composes a human-readable retry message.
func RetryMessage(wait time.Duration, attempt, maxAttempts int) string {
	return fmt.Sprintf("Retrying in %s (attempt %d/%d).",
		wait.Truncate(time.Second), attempt, maxAttempts)
}

// RetryNetworkMessage composes a human-readable retry message for network errors.
func RetryNetworkMessage(err error, wait time.Duration, attempt, maxAttempts int) string {
	return fmt.Sprintf("Connection lost (%s), retrying in %s (attempt %d/%d).",
		err, wait.Truncate(time.Second), attempt, maxAttempts)
}

// ParseIntHeader parses an integer from the named header. Returns -1 on absence or error.
func ParseIntHeader(h http.Header, key string) int {
	v := h.Get(key)
	if v == "" {
		return -1
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return -1
	}
	return n
}

// CheckAnthropicRateLimitHeaders inspects Anthropic-specific rate limit headers
// and returns a warning event if any resource is below the warning threshold.
func CheckAnthropicRateLimitHeaders(headers http.Header) (llm.Event, bool) {
	type check struct {
		resource  string
		limitKey  string
		remainKey string
		resetKey  string
	}
	checks := []check{
		{"input tokens", "anthropic-ratelimit-input-tokens-limit",
			"anthropic-ratelimit-input-tokens-remaining",
			"anthropic-ratelimit-input-tokens-reset"},
		{"output tokens", "anthropic-ratelimit-output-tokens-limit",
			"anthropic-ratelimit-output-tokens-remaining",
			"anthropic-ratelimit-output-tokens-reset"},
		{"requests", "anthropic-ratelimit-requests-limit",
			"anthropic-ratelimit-requests-remaining",
			"anthropic-ratelimit-requests-reset"},
	}

	for _, c := range checks {
		limit := ParseIntHeader(headers, c.limitKey)
		remaining := ParseIntHeader(headers, c.remainKey)
		if limit <= 0 || remaining < 0 {
			continue
		}
		if float64(remaining)/float64(limit) < WarningThreshold {
			reset := headers.Get(c.resetKey)
			msg := fmt.Sprintf(
				"Approaching Anthropic rate limit: %d of %d %s remaining (resets %s). "+
					"To increase limits, visit console.anthropic.com.",
				remaining, limit, c.resource, reset,
			)
			return llm.Event{
				Type:      llm.EventRateLimitWarning,
				RateLimit: &llm.RateLimitInfo{Message: msg},
			}, true
		}
	}
	return llm.Event{}, false
}

// CheckStandardRateLimitHeaders inspects x-ratelimit-* headers and returns
// a warning event if any resource is below the warning threshold.
func CheckStandardRateLimitHeaders(headers http.Header) (llm.Event, bool) {
	type check struct {
		resource  string
		limitKey  string
		remainKey string
		resetKey  string
	}
	checks := []check{
		{"requests", "x-ratelimit-limit-requests",
			"x-ratelimit-remaining-requests",
			"x-ratelimit-reset-requests"},
		{"tokens", "x-ratelimit-limit-tokens",
			"x-ratelimit-remaining-tokens",
			"x-ratelimit-reset-tokens"},
	}

	for _, c := range checks {
		limit := ParseIntHeader(headers, c.limitKey)
		remaining := ParseIntHeader(headers, c.remainKey)
		if limit <= 0 || remaining < 0 {
			continue
		}
		if float64(remaining)/float64(limit) < WarningThreshold {
			reset := headers.Get(c.resetKey)
			msg := fmt.Sprintf(
				"Approaching rate limit: %d of %d %s remaining (resets %s).",
				remaining, limit, c.resource, reset,
			)
			return llm.Event{
				Type:      llm.EventRateLimitWarning,
				RateLimit: &llm.RateLimitInfo{Message: msg},
			}, true
		}
	}
	return llm.Event{}, false
}

// RetryAfterOrBackoffStrategy returns a retry.Strategy that prefers the
// Retry-After header from the most recent HTTP response, falling back to
// exponential backoff with jitter.
func RetryAfterOrBackoffStrategy(
	headers *http.Header, warnings *[]llm.Event,
) retry.Strategy {
	return func(count uint) (time.Duration, bool) {
		wait := RetryWait(*headers, int(count-1))
		slog.Warn("retryable error, waiting before retry",
			"attempt", count, "wait", wait)
		*warnings = append(*warnings, llm.Event{
			Type: llm.EventRateLimitWarning,
			RateLimit: &llm.RateLimitInfo{
				WaitDuration: wait,
				Message:      RetryMessage(wait, int(count), MaxStreamRetries),
			},
		})
		return wait, false
	}
}
