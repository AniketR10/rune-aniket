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
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	readability "codeberg.org/readeck/go-readability/v2"
	"golang.org/x/net/html"
)

// Extract extracts readable content from raw bytes based on
// content type. Returns (title, content, extractMode).
func Extract(
	body []byte, contentType, pageURL string,
	maxChars int,
) (string, string, string) {
	ct := strings.ToLower(contentType)

	switch {
	case strings.Contains(ct, "application/json"):
		content := extractJSON(body)
		return "", truncate(content, maxChars), "json"

	case strings.Contains(ct, "text/plain"):
		content := string(body)
		return "", truncate(content, maxChars), "plaintext"

	case strings.Contains(ct, "text/html"),
		strings.Contains(ct, "application/xhtml"):
		title, content, mode := extractHTML(
			body, pageURL, maxChars,
		)
		return title, content, mode
	}

	// Fallback: treat as plain text.
	return "", truncate(string(body), maxChars), "plaintext"
}

func extractJSON(body []byte) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, body, "", "  "); err != nil {
		return string(body)
	}
	return buf.String()
}

func extractHTML(
	body []byte, pageURL string, maxChars int,
) (string, string, string) {
	// Tier 1: go-readability v2.
	parsed, err := url.Parse(pageURL)
	if err != nil {
		parsed = &url.URL{}
	}
	article, err := readability.FromReader(
		bytes.NewReader(body), parsed,
	)
	if err == nil && article.Node != nil {
		var buf bytes.Buffer
		if rerr := article.RenderText(&buf); rerr == nil {
			text := strings.TrimSpace(buf.String())
			if text != "" {
				return article.Title(),
					truncate(text, maxChars),
					"readability"
			}
		}
	}

	// Tier 2: custom HTML-to-markdown fallback.
	title, content := htmlToMarkdown(body)
	return title, truncate(content, maxChars), "html2md"
}

func htmlToMarkdown(body []byte) (string, string) {
	tokenizer := html.NewTokenizer(bytes.NewReader(body))
	var (
		sb        strings.Builder
		title     string
		inTitle   bool
		inScript  bool
		inStyle   bool
		inNav     bool
		inCode    bool
		inPre     bool
		listDepth int
	)

	for {
		tt := tokenizer.Next()
		if tt == html.ErrorToken {
			break
		}

		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken:
			tn, hasAttr := tokenizer.TagName()
			tag := string(tn)

			switch tag {
			case "script":
				inScript = true
			case "style":
				inStyle = true
			case "nav":
				inNav = true
			case "title":
				inTitle = true
			case "h1":
				sb.WriteString("\n# ")
			case "h2":
				sb.WriteString("\n## ")
			case "h3":
				sb.WriteString("\n### ")
			case "h4", "h5", "h6":
				sb.WriteString("\n#### ")
			case "p", "div", "section", "article":
				sb.WriteString("\n\n")
			case "br":
				sb.WriteString("\n")
			case "li":
				sb.WriteString("\n")
				for range listDepth - 1 {
					sb.WriteString("  ")
				}
				sb.WriteString("- ")
			case "ul", "ol":
				listDepth++
				sb.WriteString("\n")
			case "pre":
				inPre = true
				sb.WriteString("\n```\n")
			case "code":
				if !inPre {
					inCode = true
					sb.WriteString("`")
				}
			case "a":
				href := getAttr(tokenizer, hasAttr, "href")
				if href != "" {
					sb.WriteString("[")
				}
			case "strong", "b":
				sb.WriteString("**")
			case "em", "i":
				sb.WriteString("*")
			}

		case html.EndTagToken:
			tn, _ := tokenizer.TagName()
			tag := string(tn)

			switch tag {
			case "script":
				inScript = false
			case "style":
				inStyle = false
			case "nav":
				inNav = false
			case "title":
				inTitle = false
			case "h1", "h2", "h3", "h4", "h5", "h6":
				sb.WriteString("\n")
			case "p", "div", "section", "article":
				sb.WriteString("\n")
			case "ul", "ol":
				if listDepth > 0 {
					listDepth--
				}
			case "pre":
				inPre = false
				sb.WriteString("\n```\n")
			case "code":
				if !inPre {
					inCode = false
					sb.WriteString("`")
				}
			case "strong", "b":
				sb.WriteString("**")
			case "em", "i":
				sb.WriteString("*")
			}

		case html.TextToken:
			text := string(tokenizer.Text())
			if inScript || inStyle || inNav {
				continue
			}
			if inTitle {
				title = strings.TrimSpace(text)
				continue
			}
			if !inPre && !inCode {
				text = collapseWhitespace(text)
			}
			sb.WriteString(text)
		}
	}

	return title, normalizeOutput(sb.String())
}

func getAttr(
	tokenizer *html.Tokenizer,
	hasAttr bool, name string,
) string {
	if !hasAttr {
		return ""
	}
	for {
		key, val, more := tokenizer.TagAttr()
		if string(key) == name {
			return string(val)
		}
		if !more {
			break
		}
	}
	return ""
}

func collapseWhitespace(s string) string {
	var sb strings.Builder
	prev := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !prev {
				sb.WriteRune(' ')
				prev = true
			}
		} else {
			sb.WriteRune(r)
			prev = false
		}
	}
	return sb.String()
}

func normalizeOutput(s string) string {
	// Collapse 3+ consecutive newlines into 2.
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return strings.TrimSpace(s)
}

func truncate(s string, maxChars int) string {
	if maxChars <= 0 || len(s) <= maxChars {
		return s
	}
	return s[:maxChars] + fmt.Sprintf(
		"\n\n[Content truncated at %d characters]", maxChars,
	)
}
