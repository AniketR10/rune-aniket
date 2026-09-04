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

package webfetch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtract(t *testing.T) {
	t.Run("JSON content", func(t *testing.T) {
		body := []byte(`{"key":"value","num":42}`)
		_, content, mode := Extract(
			body, "application/json", "", 50000,
		)
		assert.Equal(t, "json", mode)
		assert.Contains(t, content, `"key": "value"`)
	})

	t.Run("plain text content", func(t *testing.T) {
		body := []byte("Hello, world!")
		_, content, mode := Extract(
			body, "text/plain", "", 50000,
		)
		assert.Equal(t, "plaintext", mode)
		assert.Equal(t, "Hello, world!", content)
	})

	t.Run("HTML with readability", func(t *testing.T) {
		body := []byte(`<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<article>
<h1>Main Article</h1>
<p>This is article content that should be extracted
by readability. It needs to be long enough for
readability to consider it as the main content of the
page. Adding more sentences helps. The go-readability
library uses heuristics to find article content.</p>
<p>Another paragraph with more content to ensure
readability detects this as the main article body.
The more content we add here, the better the extraction
will work with the readability algorithm.</p>
</article>
<nav>Navigation should be stripped</nav>
</body>
</html>`)
		title, content, mode := Extract(
			body, "text/html", "https://example.com", 50000,
		)
		assert.Contains(t, []string{"readability", "html2md"}, mode)
		if mode == "readability" {
			assert.Equal(t, "Test Page", title)
			assert.Contains(t, content, "article content")
			assert.NotContains(t, content, "Navigation should be stripped")
		}
	})

	t.Run("HTML fallback to markdown", func(t *testing.T) {
		body := []byte(`<html><head><title>My Page</title></head>
<body><h1>Header</h1><p>Paragraph</p></body></html>`)
		title, content, mode := Extract(
			body, "text/html", "https://example.com", 50000,
		)
		assert.Contains(t, []string{"readability", "html2md"}, mode)
		if mode == "html2md" {
			assert.Equal(t, "My Page", title)
			assert.Contains(t, content, "Header")
			assert.Contains(t, content, "Paragraph")
		}
	})

	t.Run("truncation", func(t *testing.T) {
		body := []byte("abcdefghij")
		_, content, _ := Extract(
			body, "text/plain", "", 5,
		)
		assert.Contains(t, content, "abcde")
		assert.Contains(t, content, "[Content truncated")
	})

	t.Run("unknown content type falls back to plain", func(t *testing.T) {
		body := []byte("raw data")
		_, content, mode := Extract(
			body, "application/octet-stream", "", 50000,
		)
		assert.Equal(t, "plaintext", mode)
		assert.Equal(t, "raw data", content)
	})
}

func TestCollapseWhitespace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no change", "hello world", "hello world"},
		{"collapse spaces", "hello   world", "hello world"},
		{"collapse tabs", "hello\t\tworld", "hello world"},
		{"collapse newlines", "hello\n\nworld", "hello world"},
		{"mixed", " \t\n hello \t\n world \n ", " hello world "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, collapseWhitespace(tt.input))
		})
	}
}

func TestTruncate(t *testing.T) {
	t.Run("no truncation needed", func(t *testing.T) {
		assert.Equal(t, "short", truncate("short", 100))
	})

	t.Run("truncates at limit", func(t *testing.T) {
		result := truncate("0123456789", 5)
		assert.Contains(t, result, "01234")
		assert.Contains(t, result, "[Content truncated at 5 characters]")
	})

	t.Run("zero max returns original", func(t *testing.T) {
		assert.Equal(t, "hello", truncate("hello", 0))
	})
}
