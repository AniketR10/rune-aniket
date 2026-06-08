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


// Package geminitools provides Gemini-specialized tools that present the
// Antigravity-native tool names and argument schemas Gemini 3 was post-trained
// to call (e.g. run_command, grep_search, codebase_search). Each tool
// translates Antigravity-shaped arguments into the schema of an underlying base
// agent.Tool and delegates execution, so the behavior is identical to the
// provider-neutral tools while the model sees the names and parameters it
// expects. Registered for the "gemini" provider via Register.
package geminitools

import (
	"context"
	"encoding/json"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/go-tui/cmd/rune-agent/agent"
)

// LLMProvider is the provider key under which these tools are registered.
const LLMProvider = "gemini"

// translator converts an Antigravity-shaped argument JSON document into the
// argument JSON expected by the underlying base tool. A translation error is
// surfaced to the model as an error ToolResult.
type translator func(arguments string) (string, error)

// specializedTool advertises an Antigravity tool definition but delegates
// execution to a base tool after translating arguments.
type specializedTool struct {
	def       llmapi.Tool
	base      agent.Tool
	translate translator
	summarize func(arguments string) string
}

func (t *specializedTool) Definition() llmapi.Tool { return t.def }

func (t *specializedTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	translated, err := t.translate(arguments)
	if err != nil {
		return agent.ToolResult{
			Content: "error: invalid arguments: " + err.Error(),
			IsError: true,
		}
	}
	return t.base.Execute(ctx, translated)
}

func (t *specializedTool) Summary(arguments string) string {
	if t.summarize != nil {
		return t.summarize(arguments)
	}
	if translated, err := t.translate(arguments); err == nil {
		return t.base.Summary(translated)
	}
	return ""
}

// remarshal decodes src into a fresh map and re-encodes it, used by
// translators that rename or drop fields.
func remarshal(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// geminiSpecializations maps a base tool name to a constructor that wraps it
// with the Antigravity-native specialization. The base tool must exist in the
// registry; specializations for absent base tools are skipped.
var geminiSpecializations = map[string]func(agent.Tool) agent.Tool{
	"bash":           runCommand,
	"search_content": grepSearch,
	"find_files":     find,
	"search_symbols": codebaseSearch,
	"outline_file":   viewFileOutline,
	"list_dir":       listDirectory,
}

// Register installs the Gemini-specialized tools as overrides for the gemini
// provider, excluding the corresponding base tool names so Gemini only sees the
// Antigravity-native set. Base tools absent from r are skipped.
func Register(r *agent.Registry) {
	for baseName, wrap := range geminiSpecializations {
		base, ok := r.Get(baseName, "")
		if !ok {
			continue
		}
		r.RegisterReplacement(LLMProvider, baseName, wrap(base))
	}
}
