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
	"encoding/json"
	"fmt"
	"strings"

	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/agentools/webfetch"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

const (
	untrustedOpen  = "<<<UNTRUSTED_CONTENT>>>"
	untrustedClose = "<<<END_UNTRUSTED_CONTENT>>>"
)

type webFetchTool struct {
	fetcher webfetch.Fetcher
}

type webFetchArgs struct {
	URL string `json:"url"`
}

// NewWebFetch creates a web fetch tool backed by the
// given Fetcher.
func NewWebFetch(f webfetch.Fetcher) agent.Tool {
	return &webFetchTool{fetcher: f}
}

func (t *webFetchTool) Definition() llmapi.Tool {
	return llmapi.Tool{
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{
			Name: "web_fetch",
			Description: `Fetch a web page and extract its text content. Returns the page title,
final URL, extraction method, and the extracted text.

The content is automatically converted from HTML to readable text. Use
this after web_search to read the full contents of a promising result,
or to fetch documentation, READMEs, or API references by URL.`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"url": map[string]any{
						"type":        "string",
						"description": "The URL to fetch.",
					},
				},
				"required":             []string{"url"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *webFetchTool) Summary(arguments string) string {
	var args webFetchArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}
	return args.URL
}

func (t *webFetchTool) Execute(
	ctx context.Context, arguments string,
) agent.ToolResult {
	var args webFetchArgs
	if err := json.Unmarshal(
		[]byte(arguments), &args,
	); err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf(
				"error: invalid arguments: %v", err,
			),
			IsError: true,
		}
	}
	if args.URL == "" {
		return agent.ToolResult{
			Content: "error: url is required",
			IsError: true,
		}
	}

	result, err := t.fetcher.Fetch(ctx, args.URL)
	if err != nil {
		return agent.ToolResult{
			Content: fmt.Sprintf(
				"error: fetch failed: %v", err,
			),
			IsError: true,
		}
	}

	return agent.ToolResult{
		Content: formatFetchResult(result),
	}
}

func formatFetchResult(r webfetch.FetchResult) string {
	var sb strings.Builder
	if r.Title != "" {
		fmt.Fprintf(&sb, "Title: %s\n", r.Title)
	}
	fmt.Fprintf(&sb, "URL: %s\n", r.URL)
	fmt.Fprintf(&sb, "Extraction: %s\n\n", r.ExtractMode)
	sb.WriteString(untrustedOpen)
	sb.WriteString("\n")
	sb.WriteString(sanitizeBoundaries(r.Content))
	sb.WriteString("\n")
	sb.WriteString(untrustedClose)
	return sb.String()
}

// sanitizeBoundaries strips boundary markers from fetched
// content to prevent escaping the untrusted block.
func sanitizeBoundaries(content string) string {
	s := strings.ReplaceAll(content, untrustedOpen, "")
	s = strings.ReplaceAll(s, untrustedClose, "")
	return s
}
