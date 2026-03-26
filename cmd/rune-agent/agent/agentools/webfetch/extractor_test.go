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
