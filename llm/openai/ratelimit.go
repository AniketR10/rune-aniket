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

package openai

import (
	"errors"
	"net/http"
	"strings"
	"time"

	oai "github.com/openai/openai-go/v2"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/retry"
	"unstable.build/rune/llm/ratelimit"
)

// Re-export shared constants so existing call sites within this package compile.
const maxStreamRetries = ratelimit.MaxStreamRetries

// permanentAPIErrorCodes lists API error codes that indicate a permanent
// condition (e.g. billing issues) which should not be retried even though
// the HTTP status might otherwise be retryable.
var permanentAPIErrorCodes = map[string]bool{
	"insufficient_quota": true, // OpenAI: billing exhausted
}

// isRetryableError reports whether err represents a transient error that
// should be retried. Uses the OpenAI SDK error types for extraction.
func isRetryableError(err error) bool {
	if override, ok := shouldRetryHeaderFromError(err); ok {
		return override
	}
	if permanentAPIErrorCodes[apiErrorCodeFromError(err)] {
		return false
	}
	if ratelimit.IsTransientNetworkError(err) {
		return true
	}
	status := httpStatusFromError(err)
	switch {
	case status == http.StatusRequestTimeout,
		status == http.StatusConflict,
		status == http.StatusTooManyRequests:
		return true
	case status >= http.StatusInternalServerError:
		return true
	}
	return false
}

// shouldRetryHeaderFromError extracts the x-should-retry header from the
// HTTP response attached to an OpenAI SDK error.
func shouldRetryHeaderFromError(err error) (bool, bool) {
	var apiErr *oai.Error
	if errors.As(err, &apiErr) && apiErr.Response != nil {
		if v := apiErr.Response.Header.Get("x-should-retry"); v != "" {
			return v == "true", true
		}
	}
	type shouldRetryer interface {
		ShouldRetry() (bool, bool)
	}
	var sr shouldRetryer
	if errors.As(err, &sr) {
		return sr.ShouldRetry()
	}
	return false, false
}

// isTransientNetworkError delegates to the shared ratelimit package.
var isTransientNetworkError = ratelimit.IsTransientNetworkError

// isRetryableStreamError delegates to the shared ratelimit package.
var isRetryableStreamError = ratelimit.IsRetryableStreamError

// retryNetworkMessage delegates to the shared ratelimit package.
func retryNetworkMessage(err error, wait time.Duration, attempt, maxAttempts int) string {
	return ratelimit.RetryNetworkMessage(err, wait, attempt, maxAttempts)
}

// retryStreamMessage delegates to the shared ratelimit package.
func retryStreamMessage(err error, wait time.Duration, attempt, maxAttempts int) string {
	return ratelimit.RetryStreamMessage(err, wait, attempt, maxAttempts)
}

// isToolCallParseError reports whether err is an Ollama server-side error
// where the model output could not be parsed as a valid tool call.
func isToolCallParseError(err error) bool {
	return strings.Contains(err.Error(), "error parsing tool call")
}

// httpStatusFromError extracts the HTTP status code from an OpenAI SDK error.
func httpStatusFromError(err error) int {
	var apiErr *oai.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	type statusCoder interface {
		StatusCode() int
	}
	var sc statusCoder
	if errors.As(err, &sc) {
		return sc.StatusCode()
	}
	return 0
}

// apiErrorCodeFromError extracts the API error code from an OpenAI SDK error.
func apiErrorCodeFromError(err error) string {
	var apiErr *oai.Error
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	type apiErrorCoder interface {
		APIErrorCode() string
	}
	var ac apiErrorCoder
	if errors.As(err, &ac) {
		return ac.APIErrorCode()
	}
	return ""
}

// retryWait delegates to the shared ratelimit package.
func retryWait(headers http.Header, attempt int) time.Duration {
	return ratelimit.RetryWait(headers, attempt)
}

// ToolCallParseError indicates the model produced output that could not be
// parsed as a valid tool call.
type ToolCallParseError struct {
	Cause error
}

func (e *ToolCallParseError) Error() string {
	return "model produced an invalid tool call (mixed reasoning text with tool arguments); consider using a model with better tool-use support"
}

func (e *ToolCallParseError) Unwrap() error {
	return e.Cause
}

// checkAnthropicRateLimitHeaders delegates to the shared ratelimit package.
func checkAnthropicRateLimitHeaders(headers http.Header) (llmapi.Event, bool) {
	return ratelimit.CheckAnthropicRateLimitHeaders(headers)
}

// checkStandardRateLimitHeaders delegates to the shared ratelimit package.
func checkStandardRateLimitHeaders(headers http.Header) (llmapi.Event, bool) {
	return ratelimit.CheckStandardRateLimitHeaders(headers)
}

// retryAfterOrBackoffStrategy delegates to the shared ratelimit package.
func retryAfterOrBackoffStrategy(
	headers *http.Header, warnings *[]llmapi.Event,
) retry.Strategy {
	return ratelimit.RetryAfterOrBackoffStrategy(headers, warnings)
}

// isAnthropicURL reports whether the configured base URL points to Anthropic's API.
func isAnthropicURL(baseURL string) bool {
	return strings.Contains(baseURL, "anthropic.com")
}
