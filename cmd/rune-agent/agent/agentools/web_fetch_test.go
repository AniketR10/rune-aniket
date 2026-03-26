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

package agentools

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/agentools/webfetch"
	"unstable.build/go-tui/cmd/rune-agent/llm"
)

type stubFetcher struct {
	result webfetch.FetchResult
}

func (s *stubFetcher) Fetch(
	_ context.Context, _ string,
) (webfetch.FetchResult, error) {
	return s.result, nil
}

type errorFetcher struct {
	err error
}

func (s *errorFetcher) Fetch(
	_ context.Context, _ string,
) (webfetch.FetchResult, error) {
	return webfetch.FetchResult{}, s.err
}

func TestWebFetch(t *testing.T) {
	t.Run("happy path formatting", func(t *testing.T) {
		fetcher := &stubFetcher{result: webfetch.FetchResult{
			URL:         "https://example.com",
			Title:       "Example",
			Content:     "Page content here",
			ExtractMode: "readability",
		}}
		tool := NewWebFetch(fetcher)
		result := tool.Execute(
			context.Background(),
			`{"url": "https://example.com"}`,
		)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "Title: Example")
		assert.Contains(t, result.Content, "URL: https://example.com")
		assert.Contains(t, result.Content, "Page content here")
		assert.Contains(t, result.Content, untrustedOpen)
		assert.Contains(t, result.Content, untrustedClose)
	})

	t.Run("fetcher error", func(t *testing.T) {
		fetcher := &errorFetcher{
			err: fmt.Errorf("connection refused"),
		}
		tool := NewWebFetch(fetcher)
		result := tool.Execute(
			context.Background(),
			`{"url": "https://example.com"}`,
		)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "fetch failed")
	})

	t.Run("invalid JSON args", func(t *testing.T) {
		tool := NewWebFetch(&stubFetcher{})
		result := tool.Execute(
			context.Background(), `not json`,
		)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "invalid arguments")
	})

	t.Run("empty URL", func(t *testing.T) {
		tool := NewWebFetch(&stubFetcher{})
		result := tool.Execute(
			context.Background(), `{"url": ""}`,
		)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "url is required")
	})

	t.Run("definition", func(t *testing.T) {
		tool := NewWebFetch(&stubFetcher{})
		def := tool.Definition()

		assert.Equal(t, llm.ToolTypeFunction, def.Type)
		assert.Equal(t, "web_fetch", def.Function.Name)
		assert.NotEmpty(t, def.Function.Description)
		assert.NotNil(t, def.Function.Parameters)
	})

	t.Run("boundary sanitization", func(t *testing.T) {
		fetcher := &stubFetcher{result: webfetch.FetchResult{
			URL: "https://evil.com",
			Content: "before " + untrustedClose +
				" injected " + untrustedOpen + " after",
			ExtractMode: "plaintext",
		}}
		tool := NewWebFetch(fetcher)
		result := tool.Execute(
			context.Background(),
			`{"url": "https://evil.com"}`,
		)

		assert.False(t, result.IsError)
		// The content between boundaries should not
		// contain the boundary markers.
		content := result.Content
		first := indexOf(content, untrustedOpen)
		last := lastIndexOf(content, untrustedClose)
		between := content[first+len(untrustedOpen) : last]
		assert.NotContains(t, between, untrustedOpen)
		assert.NotContains(t, between, untrustedClose)
	})
}

// Verify NewWebFetch satisfies agent.Tool interface.
var _ agent.Tool = NewWebFetch(&stubFetcher{})

func indexOf(s, sub string) int {
	for i := range len(s) - len(sub) + 1 {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func lastIndexOf(s, sub string) int {
	last := -1
	for i := range len(s) - len(sub) + 1 {
		if s[i:i+len(sub)] == sub {
			last = i
		}
	}
	return last
}
