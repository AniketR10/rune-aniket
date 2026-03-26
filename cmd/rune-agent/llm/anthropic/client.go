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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	ant "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/ssestream"
	"unstable.build/go-tui/cmd/rune-agent/llm"
	"unstable.build/go-tui/cmd/rune-agent/llm/ratelimit"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/retry"
)

// Config holds the configuration for the Anthropic client.
type Config struct {
	Model       string
	MaxTokens   int
	Temperature float64
	TopP        float64
	DebugHTTP   bool
	// BaseURL overrides the default Anthropic API endpoint.
	BaseURL string

	// ReasoningEffort maps to Anthropic's OutputConfig.Effort.
	// Valid values: "none", "minimal", "low", "medium", "high", "xhigh" (OpenAI),
	// "max" (Anthropic Opus 4.6 only).
	ReasoningEffort string
	// EnableThinking enables adaptive extended thinking. Claude decides
	// dynamically when and how much to think. Recommended for 4.6 models.
	EnableThinking bool
	// ResponseFormat, when set, maps to Anthropic's OutputConfig.Format
	// for structured JSON output.
	ResponseFormat *llm.ResponseFormat
	// CacheControl configures prompt caching. Valid values:
	// "ephemeral" (5m default TTL), "5m", "1h", or "" (disabled).
	CacheControl string
}

type client struct {
	config        Config
	anthropic     ant.Client
	contextWindow int
}

// NewClient returns an llm.Service backed by the native Anthropic Messages API
// with prompt caching support.
func NewClient(token string, config Config, contextWindows map[string]int) llm.Service {
	return NewClientWithHTTP(token, config, contextWindows, nil)
}

// CreateCompletion starts a streaming completion using the native Anthropic
// Messages API. Prompt caching is automatically applied to the system prompt
// and tool definitions.
func (c *client) CreateCompletion(
	ctx context.Context, request llm.Request,
) (iterator.Iterator[llm.Event], error) {
	// Check token count against context window with 5% safety margin.
	// Use the caller-supplied token count when available (the agent loop
	// pre-computes this from provider-reported usage), avoiding a redundant
	// HTTP round-trip to the CountTokens API on every turn.
	if c.contextWindow > 0 {
		count := request.TokenCount
		if count == 0 {
			var err error
			count, err = c.CountTokens(request.Messages)
			if err != nil {
				count = 0
			}
		}
		if count > 0 {
			maxAllowed := int(float64(c.contextWindow) * 0.95)
			if count > maxAllowed {
				return nil, &llm.ErrContextWindowExceeded{Count: count, Max: c.contextWindow}
			}
		}
	}

	// Resolve effective effort: request-level takes precedence over config-level.
	effort := string(request.ReasoningEffort)
	if effort == "" {
		effort = c.config.ReasoningEffort
	}

	// Normalize effort against model capabilities. Unsupported levels are
	// dropped (empty string) so the provider default applies.
	var effortWarnings []llm.Event
	normalized, effortWarn := NormalizeEffort(c.config.Model, effort)
	request.ReasoningEffort = llm.ReasoningEffort(normalized)
	if effortWarn != "" {
		effortWarnings = append(effortWarnings, llm.Event{
			Type: llm.EventRateLimitWarning,
			RateLimit: &llm.RateLimitInfo{
				Message: effortWarn,
			},
		})
	}

	// Build request parameters.
	params := anthropicParamsFromRequest(request, c.config)

	// Apply prompt caching breakpoints on system + tools.
	applyCacheBreakpoints(&params, c.config.CacheControl)

	// Capture response headers via middleware for rate limit inspection.
	var capturedHeaders http.Header
	headerMiddleware := option.WithMiddleware(
		func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
			resp, err := next(req)
			if resp != nil {
				capturedHeaders = resp.Header.Clone()
			}
			return resp, err
		},
	)

	// Retry on transient errors.
	var warnings []llm.Event
	var stream *ssestream.Stream[ant.MessageStreamEventUnion]

	retryStrategy := retry.CombinedStrategy(
		retry.LimitStrategy(ratelimit.MaxStreamRetries+1),
		ratelimit.RetryAfterOrBackoffStrategy(&capturedHeaders, &warnings),
	)

	retryErr := retry.Retry(ctx, retryStrategy, func(_ context.Context) (bool, error) {
		capturedHeaders = nil
		stream = c.anthropic.Messages.NewStreaming(ctx, params,
			headerMiddleware, option.WithMaxRetries(0))

		err := stream.Err()
		if err == nil {
			return false, nil
		}
		if !isRetryableError(err) {
			return false, err
		}
		_ = stream.Close()
		return true, err
	})
	if retryErr != nil {
		if len(warnings) == 0 {
			if stream != nil {
				_ = stream.Close()
			}
			return nil, retryErr
		}
	}

	newStream := func() *ssestream.Stream[ant.MessageStreamEventUnion] {
		capturedHeaders = nil
		return c.anthropic.Messages.NewStreaming(ctx, params,
			headerMiddleware, option.WithMaxRetries(0))
	}

	// Prepend effort normalization warnings before any retry warnings.
	if len(effortWarnings) > 0 {
		warnings = append(effortWarnings, warnings...)
	}

	return &streamIterator{
		stream:           stream,
		pendingWarnings:  warnings,
		capturedHeaders:  capturedHeaders,
		newStream:        newStream,
		midStreamRetries: ratelimit.MaxMidStreamRetries,
	}, nil
}

// CountTokens returns the token count for the given messages using the
// Anthropic CountTokens API, which provides exact counts for the model's
// tokenizer. Falls back to a character-based estimate on API errors.
func (c *client) CountTokens(msgs []llm.Message) (int, error) {
	system, messages := convertMessages(msgs)

	params := ant.MessageCountTokensParams{
		Model:    ant.Model(c.config.Model),
		Messages: messages,
	}
	if len(system) > 0 {
		params.System = ant.MessageCountTokensParamsSystemUnion{
			OfTextBlockArray: system,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := c.anthropic.Messages.CountTokens(ctx, params)
	if err != nil {
		slog.Warn("anthropic CountTokens API failed, falling back to estimate",
			"error", err)
		return estimateTokens(msgs), nil
	}
	return int(result.InputTokens), nil
}

// estimateTokens provides a rough character-based token estimate as a fallback
// when the CountTokens API is unavailable.
func estimateTokens(msgs []llm.Message) int {
	var total int
	for _, msg := range msgs {
		total += len(msg.Content) / 4
		total += len(msg.ReasoningContent) / 4
		for _, p := range msg.MultiContent {
			total += len(p.Text) / 4
		}
		for _, tc := range msg.ToolCalls {
			total += len(tc.Function.Arguments) / 4
			total += len(tc.Function.Name) / 4
		}
		total += 4 // per-message overhead
	}
	return total
}

// ContextWindow returns the context window size for the configured model.
func (c *client) ContextWindow() int {
	return c.contextWindow
}

// isRetryableError reports whether err represents a transient Anthropic API error.
func isRetryableError(err error) bool {
	// Check x-should-retry header from Anthropic response.
	var apiErr *ant.Error
	if errors.As(err, &apiErr) && apiErr.Response != nil {
		if v := apiErr.Response.Header.Get("x-should-retry"); v != "" {
			return v == "true"
		}
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

// httpStatusFromError extracts the HTTP status code from an Anthropic SDK error.
func httpStatusFromError(err error) int {
	var apiErr *ant.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	return 0
}

// debugMiddleware returns an option.Middleware that logs HTTP request/response details.
func debugMiddleware() option.Middleware {
	return func(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
		slog.Debug("anthropic HTTP request",
			"method", req.Method,
			"url", req.URL.String(),
			"content_length", req.ContentLength,
		)
		resp, err := next(req)
		if resp != nil {
			slog.Debug("anthropic HTTP response",
				"status", resp.StatusCode,
				"content_type", resp.Header.Get("Content-Type"),
				"request_id", resp.Header.Get("Request-Id"),
			)
		}
		if err != nil {
			slog.Debug("anthropic HTTP error", "error", err)
		}
		return resp, err
	}
}

// NewClientWithHTTP creates an llm.Service backed by the Anthropic Messages API
// using the supplied HTTP client. Pass nil to use the default transport.
// Intended for tests and benchmarks that need an in-process transport without
// starting a real server.
func NewClientWithHTTP(token string, config Config, contextWindows map[string]int, httpClient *http.Client) llm.Service {
	if config.Model == "" {
		panic("Config.Model cannot be empty")
	}
	opts := []option.RequestOption{
		option.WithAPIKey(token),
		option.WithMaxRetries(0), // We handle retries ourselves.
	}
	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(config.BaseURL))
	}
	if httpClient != nil {
		opts = append(opts, option.WithHTTPClient(httpClient))
	}
	if config.DebugHTTP {
		opts = append(opts, option.WithMiddleware(debugMiddleware()))
	}
	return &client{
		config:        config,
		anthropic:     ant.NewClient(opts...),
		contextWindow: contextWindows[config.Model],
	}
}

// Verify at compile time that client implements llm.Service.
var _ llm.Service = (*client)(nil)

// ClientConstructor is the function signature for creating an Anthropic
// llm.Service. Tests may substitute a stub.
type ClientConstructor func(token string, cfg Config, models map[string]int) llm.Service

// Verify NewClient matches ClientConstructor signature.
var _ ClientConstructor = NewClient

// FormatUsageLog returns a formatted string with token usage including
// cache metrics for structured logging.
func FormatUsageLog(usage llm.Usage) string {
	return fmt.Sprintf("sent=%d received=%d cached=%d",
		usage.TokensSent, usage.TokensReceived, usage.TokensCached)
}
