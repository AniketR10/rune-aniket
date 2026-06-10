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

package anthropic

import (
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

// LLMProvider identifies the Anthropic provider in the model registry.
const LLMProvider = "anthropic"

const (
	// ClaudeFable5 is Anthropic's Claude Fable 5 model — its most capable
	// widely released model.
	ClaudeFable5 = "claude-fable-5"
	// ClaudeOpus4Dot8 is Anthropic's Claude Opus 4.8 model.
	ClaudeOpus4Dot8 = "claude-opus-4-8"
	// ClaudeOpus4Dot7 is Anthropic's Claude Opus 4.7 model.
	ClaudeOpus4Dot7 = "claude-opus-4-7"
	// ClaudeOpus4Dot6 is Anthropic's Claude Opus 4.6 model.
	ClaudeOpus4Dot6 = "claude-opus-4-6"
	// ClaudeSonnet4Dot6 is Anthropic's Claude Sonnet 4.6 model.
	ClaudeSonnet4Dot6 = "claude-sonnet-4-6"
	// ClaudeHaiku4Dot5 is Anthropic's Claude Haiku 4.5 model.
	ClaudeHaiku4Dot5 = "claude-haiku-4-5"
	// ClaudeOpus4Dot5 is Anthropic's Claude Opus 4.5 model.
	ClaudeOpus4Dot5 = "claude-opus-4-5"
	// ClaudeSonnet4Dot5 is Anthropic's Claude Sonnet 4.5 model.
	ClaudeSonnet4Dot5 = "claude-sonnet-4-5"
	// ClaudeOpus4Dot1 is Anthropic's Claude Opus 4.1 model.
	ClaudeOpus4Dot1 = "claude-opus-4-1"
	// ClaudeSonnet4 is Anthropic's Claude Sonnet 4.0 model.
	ClaudeSonnet4 = "claude-sonnet-4-0"
	// ClaudeOpus4 is Anthropic's Claude Opus 4.0 model.
	ClaudeOpus4 = "claude-opus-4-0"
	// ClaudeOpus3 is Anthropic's Claude 3 Opus model.
	ClaudeOpus3 = "claude-3-opus"
	// ClaudeSonnet3 is Anthropic's Claude 3 Sonnet model.
	ClaudeSonnet3 = "claude-3-sonnet"
	// ClaudeHaiku3 is Anthropic's Claude 3 Haiku model.
	ClaudeHaiku3 = "claude-3-haiku"
)

// AvailableModels returns a map from model identifier -> nominal maximum
// context window (tokens). These values are collected from provider
// documentation and public release notes as of early 2026. They are a
// convenience for client-side capacity checks — always verify with the
// provider at runtime for account- or region-specific limits.
func AvailableModels() map[string]int {
	return map[string]int{
		ClaudeFable5:      1000000,
		ClaudeOpus4Dot8:   1000000,
		ClaudeOpus4Dot7:   1000000,
		ClaudeOpus4Dot6:   1000000,
		ClaudeSonnet4Dot6: 1000000,
		ClaudeHaiku4Dot5:  200000,
		ClaudeOpus4Dot5:   200000,
		ClaudeSonnet4Dot5: 200000,
		ClaudeOpus4Dot1:   200000,
		ClaudeSonnet4:     200000,
		ClaudeOpus4:       200000,
		ClaudeOpus3:       200000,
		ClaudeSonnet3:     200000,
		ClaudeHaiku3:      200000,
	}
}

// SupportsAdaptiveThinking reports whether the given model supports adaptive
// extended thinking. Currently only 4.6 models support the adaptive mode.
func SupportsAdaptiveThinking(model string) bool {
	switch model {
	case ClaudeOpus4Dot6, ClaudeSonnet4Dot6, ClaudeOpus4Dot7, ClaudeOpus4Dot8, ClaudeFable5:
		return true
	default:
		return false
	}
}

// SupportsEffort reports whether the given model supports the
// OutputConfig.Effort parameter. Claude 4+ models support it;
// Claude 3 models do not.
func SupportsEffort(model string) bool {
	return !strings.HasPrefix(model, "claude-3")
}

// claude4Efforts are the effort levels supported by Claude 4.0–4.5 models.
var claude4Efforts = map[string]bool{
	"low":    true,
	"medium": true,
	"high":   true,
}

// claude46Efforts are the effort levels supported by Claude 4.6 models.
// These models additionally support "xhigh" and "max".
var claude46Efforts = map[string]bool{
	"low":    true,
	"medium": true,
	"high":   true,
	"xhigh":  true,
	"max":    true,
}

// NormalizeEffort validates the requested effort level against the given
// model's capabilities. It returns the effort to use (empty string means
// omit the parameter entirely) and a human-readable warning when the
// requested effort is not supported by the model.
func NormalizeEffort(model, effort string) (normalized string, warning string) {
	if effort == "" {
		return "", ""
	}

	if !SupportsEffort(model) {
		return "", fmt.Sprintf(
			"Effort %q is not supported by %s; using model default instead.", effort, model)
	}

	var supported map[string]bool
	if SupportsAdaptiveThinking(model) {
		supported = claude46Efforts
	} else {
		supported = claude4Efforts
	}

	if supported[effort] {
		return effort, ""
	}

	return "", fmt.Sprintf(
		"Effort %q is not supported by %s; using model default instead.", effort, model)
}

// ModelEntries returns the static catalog of Anthropic models in this
// provider, suitable for registering with an llmrouter.Router via
// llmrouter.WithModels.
func ModelEntries() []llmapi.ModelEntry {
	avail := AvailableModels()
	out := make([]llmapi.ModelEntry, 0, len(avail))
	for name, ctxWindow := range avail {
		out = append(out, llmapi.ModelEntry{
			Name:          name,
			Provider:      LLMProvider,
			ContextWindow: ctxWindow,
		})
	}
	return out
}

// FlagshipModel returns the provider's top model identifier. It is
// deterministic, unlike iterating ModelEntries() whose order is
// map-random.
func FlagshipModel() string { return ClaudeFable5 }
