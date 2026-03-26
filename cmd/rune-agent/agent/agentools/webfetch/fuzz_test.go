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
	"strings"
	"testing"
)

// FuzzExtract exercises the full extraction pipeline with
// arbitrary input to catch panics, infinite loops, and
// out-of-bounds errors across all content-type tiers.
func FuzzExtract(f *testing.F) {
	// Seed corpus covering each extraction path.
	f.Add(
		[]byte(`{"key": "value"}`),
		"application/json",
		"https://example.com",
	)
	f.Add(
		[]byte("Hello, world!"),
		"text/plain",
		"https://example.com",
	)
	f.Add(
		[]byte(`<html><head><title>T</title></head>
<body><p>Content</p></body></html>`),
		"text/html",
		"https://example.com",
	)
	f.Add(
		[]byte(`<html><body>
<script>alert('xss')</script>
<style>.x{color:red}</style>
<h1>Title</h1><p>Body</p>
<ul><li>Item 1</li><li>Item 2</li></ul>
<pre><code>code block</code></pre>
</body></html>`),
		"text/html; charset=utf-8",
		"https://example.com/page",
	)
	f.Add(
		[]byte("binary\x00data\xff\xfe"),
		"application/octet-stream",
		"",
	)
	f.Add(
		[]byte(`<!DOCTYPE html><html><body>
<article><p>`+longParagraph()+`</p></article>
</body></html>`),
		"text/html",
		"https://example.com/article",
	)
	f.Add(
		[]byte("{invalid json"),
		"application/json",
		"https://example.com",
	)
	f.Add(
		[]byte("<not>valid<html"),
		"text/html",
		"https://example.com",
	)

	f.Fuzz(func(
		t *testing.T, body []byte, ct, pageURL string,
	) {
		// Must not panic regardless of input.
		title, content, mode := Extract(
			body, ct, pageURL, 5000,
		)

		// Mode must always be one of the known values.
		switch mode {
		case "json", "plaintext", "readability", "html2md":
		default:
			t.Errorf("unexpected extract mode: %q", mode)
		}

		// Content must be truncated within limits.
		// 5000 chars + truncation notice ≈ max ~5060.
		if len(content) > 5100 {
			t.Errorf(
				"content exceeds limit: %d chars",
				len(content),
			)
		}

		// Title should never be excessively large.
		if len(title) > len(body)+100 {
			t.Errorf(
				"title larger than input: %d vs %d",
				len(title), len(body),
			)
		}
	})
}

// FuzzHTMLToMarkdown exercises the custom HTML-to-markdown
// converter with arbitrary HTML to catch panics.
func FuzzHTMLToMarkdown(f *testing.F) {
	f.Add([]byte("<html><body><p>Hello</p></body></html>"))
	f.Add([]byte("<h1>Title</h1><h2>Sub</h2>"))
	f.Add([]byte("<ul><li>a</li><li>b</li></ul>"))
	f.Add([]byte("<pre><code>x</code></pre>"))
	f.Add([]byte("<script>alert(1)</script><p>ok</p>"))
	f.Add([]byte("<a href='http://x.com'>link</a>"))
	f.Add([]byte("<strong><em>bold italic</em></strong>"))
	f.Add([]byte(""))
	f.Add([]byte("<"))
	f.Add([]byte("<div><div><div>deep</div></div></div>"))

	f.Fuzz(func(t *testing.T, body []byte) {
		// Must not panic.
		_, _ = htmlToMarkdown(body)
	})
}

// FuzzValidateURL exercises URL validation with arbitrary
// strings to catch panics in the SSRF hostname/scheme checks.
func FuzzValidateURL(f *testing.F) {
	f.Add("https://example.com/page")
	f.Add("http://localhost/secret")
	f.Add("ftp://example.com")
	f.Add("file:///etc/passwd")
	f.Add("http://169.254.169.254/latest/meta-data")
	f.Add("http://metadata.google.internal/v1")
	f.Add("http://[::1]/path")
	f.Add("http://myhost.local/path")
	f.Add("")
	f.Add("not-a-url")
	f.Add("http:///no-host")

	f.Fuzz(func(t *testing.T, rawURL string) {
		// Must not panic. Errors are expected for many inputs.
		_ = validateURL(rawURL, false)
	})
}

// FuzzCollapseWhitespace exercises whitespace collapsing
// with arbitrary strings.
func FuzzCollapseWhitespace(f *testing.F) {
	f.Add("hello world")
	f.Add("  multiple   spaces  ")
	f.Add("\t\n\r mixed")
	f.Add("")
	f.Add("no-whitespace")

	f.Fuzz(func(t *testing.T, input string) {
		result := collapseWhitespace(input)
		// Result must never contain consecutive whitespace.
		for i := range len(result) - 1 {
			if isWS(result[i]) && isWS(result[i+1]) {
				t.Errorf(
					"consecutive whitespace at %d in %q",
					i, result,
				)
				break
			}
		}
	})
}

func isWS(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func longParagraph() string {
	s := "This is a long paragraph for readability. "
	var sb strings.Builder
	for range 20 {
		sb.WriteString(s)
	}
	return sb.String()
}
