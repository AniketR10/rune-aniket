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

package agent

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

func TestTruncateMiddle(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxBytes int
		check    func(t *testing.T, result string)
	}{
		{
			name:     "short content passes through unchanged",
			input:    "hello world",
			maxBytes: 100,
			check: func(t *testing.T, result string) {
				assert.Equal(t, "hello world", result)
			},
		},
		{
			name:     "exact limit passes through unchanged",
			input:    "hello",
			maxBytes: 5,
			check: func(t *testing.T, result string) {
				assert.Equal(t, "hello", result)
			},
		},
		{
			name:     "long content is truncated to within limit",
			input:    strings.Repeat("x", 1000),
			maxBytes: 200,
			check: func(t *testing.T, result string) {
				assert.LessOrEqual(t, len(result), 200+100, "result should be near the configured limit")
				assert.Contains(t, result, "bytes truncated")
				assert.True(t, strings.HasPrefix(result, "Total output lines:"))
			},
		},
		{
			name:     "marker includes correct byte count",
			input:    strings.Repeat("a", 500),
			maxBytes: 200,
			check: func(t *testing.T, result string) {
				// Extract the truncated byte count from the marker.
				idx := strings.Index(result, "…[")
				require.NotEqual(t, -1, idx)
				end := strings.Index(result[idx:], " bytes truncated]")
				require.NotEqual(t, -1, end)
				countStr := result[idx+len("…[") : idx+end]
				// The truncated bytes + kept bytes should equal original length.
				var truncated int
				_, err := fmt.Sscanf(countStr, "%d", &truncated)
				require.NoError(t, err)
				assert.Greater(t, truncated, 0)
			},
		},
		{
			name:     "UTF-8 multibyte characters are not split",
			input:    strings.Repeat("日本語テスト", 100), // 6 chars × 3 bytes each × 100 = 1800 bytes
			maxBytes: 200,
			check: func(t *testing.T, result string) {
				assert.True(t, utf8.ValidString(result), "result must be valid UTF-8")
				assert.Contains(t, result, "bytes truncated")
			},
		},
		{
			name:     "preserves prefix and suffix content",
			input:    "START" + strings.Repeat("m", 1000) + "END",
			maxBytes: 200,
			check: func(t *testing.T, result string) {
				// After the header, the prefix should start with "START".
				lines := strings.SplitN(result, "\n\n", 2)
				require.Len(t, lines, 2)
				body := lines[1]
				assert.True(t, strings.HasPrefix(body, "START"), "should preserve prefix")
				assert.True(t, strings.HasSuffix(body, "END"), "should preserve suffix")
			},
		},
		{
			name:     "includes total output lines header",
			input:    "line1\nline2\nline3\n" + strings.Repeat("x", 1000),
			maxBytes: 200,
			check: func(t *testing.T, result string) {
				assert.True(t, strings.HasPrefix(result, "Total output lines:"))
			},
		},
		{
			name:     "very small maxBytes still produces valid output",
			input:    strings.Repeat("x", 100),
			maxBytes: 10,
			check: func(t *testing.T, result string) {
				assert.True(t, utf8.ValidString(result))
				assert.Contains(t, result, "truncated")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateMiddle(tt.input, tt.maxBytes)
			tt.check(t, result)
		})
	}
}

func TestTruncateLine(t *testing.T) {
	marker := truncatedLineMarker // " [truncated line]"

	tests := []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{
			name:     "short line passes through unchanged",
			input:    "hello world",
			max:      100,
			expected: "hello world",
		},
		{
			name:     "exact limit passes through unchanged",
			input:    "hello",
			max:      5,
			expected: "hello",
		},
		{
			name:     "long line is truncated with marker",
			input:    strings.Repeat("x", 600),
			max:      500,
			expected: strings.Repeat("x", 500-len(marker)) + marker,
		},
		{
			name:  "multibyte characters are not split",
			input: strings.Repeat("日", 200), // 3 bytes each = 600 bytes
			max:   500,
			// (500-17)=483 bytes budget; 483/3=161 full runes = 483 bytes + 17 marker = 500
			expected: strings.Repeat("日", 161) + marker,
		},
		{
			name:     "very small max still produces valid output",
			input:    strings.Repeat("x", 100),
			max:      20,
			expected: strings.Repeat("x", 20-len(marker)) + marker, // 3 bytes + marker
		},
		{
			name:     "empty string passes through",
			input:    "",
			max:      500,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TruncateLine(tt.input, tt.max)
			assert.True(t, utf8.ValidString(result), "result must be valid UTF-8")
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCapToolResult(t *testing.T) {
	const maxOutput = 1000

	tests := []struct {
		name  string
		in    ToolResult
		check func(t *testing.T, r ToolResult)
	}{
		{
			name: "oversized success content is truncated",
			in: ToolResult{
				Content: strings.Repeat("x", maxOutput*4),
				IsError: false,
			},
			check: func(t *testing.T, r ToolResult) {
				assert.LessOrEqual(t, len(r.Content), maxOutput+100)
				assert.Contains(t, r.Content, "bytes truncated")
			},
		},
		{
			name: "oversized error content is truncated",
			in: ToolResult{
				Content: strings.Repeat("e", maxOutput*4),
				IsError: true,
			},
			check: func(t *testing.T, r ToolResult) {
				assert.LessOrEqual(t, len(r.Content), maxOutput+100)
				assert.Contains(t, r.Content, "bytes truncated")
			},
		},
		{
			name: "small content is untouched",
			in: ToolResult{
				Content: "ok",
			},
			check: func(t *testing.T, r ToolResult) {
				assert.Equal(t, "ok", r.Content)
			},
		},
		{
			name: "oversized image part is replaced with placeholder",
			in: ToolResult{
				Content: "summary",
				MultiContent: []llmapi.ContentPart{
					{Type: llmapi.ContentPartTypeText, Text: "summary"},
					{Type: llmapi.ContentPartTypeImageURL, ImageURL: strings.Repeat("d", MaxToolResultBytes+1)},
				},
			},
			check: func(t *testing.T, r ToolResult) {
				require.Len(t, r.MultiContent, 2)
				assert.Equal(t, llmapi.ContentPartTypeText, r.MultiContent[0].Type)
				assert.Equal(t, "summary", r.MultiContent[0].Text)
				assert.Equal(t, llmapi.ContentPartTypeText, r.MultiContent[1].Type)
				assert.Empty(t, r.MultiContent[1].ImageURL)
				assert.Equal(t,
					fmt.Sprintf("[image omitted: %d bytes exceeds limit]", MaxToolResultBytes+1),
					r.MultiContent[1].Text)
			},
		},
		{
			name: "small image part is untouched",
			in: ToolResult{
				MultiContent: []llmapi.ContentPart{
					{Type: llmapi.ContentPartTypeImageURL, ImageURL: "data:image/png;base64,AAAA"},
				},
			},
			check: func(t *testing.T, r ToolResult) {
				require.Len(t, r.MultiContent, 1)
				assert.Equal(t, llmapi.ContentPartTypeImageURL, r.MultiContent[0].Type)
				assert.Equal(t, "data:image/png;base64,AAAA", r.MultiContent[0].ImageURL)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.in
			capToolResult(&r, maxOutput)
			tt.check(t, r)
		})
	}
}
