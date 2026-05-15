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


package openai

import (
	"errors"
	"net/http"
	"strings"
	"time"

	oai "github.com/openai/openai-go/v2"
	"github.com/unstablebuild/rune-go-sdk/retry"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/go-tui/llm/ratelimit"
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
