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


package openai

import (
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

// LLMProvider identifies the OpenAI provider in the model registry.
const LLMProvider = "openai"

const (
	// GPT5Dot5 is the GPT-5.5 model.
	GPT5Dot5 = "gpt-5.5"
	// GPT5Dot4 is the GPT-5.4 model.
	GPT5Dot4 = "gpt-5.4"
	// GPT5Dot4Pro is the GPT-5.4 Pro model.
	GPT5Dot4Pro = "gpt-5.4-pro"
	// GPT5Dot4Mini is the GPT-5.4 Mini model.
	GPT5Dot4Mini = "gpt-5.4-mini"
	// GPT5Dot4Nano is the GPT-5.4 Nano model.
	GPT5Dot4Nano = "gpt-5.4-nano"
	// GPT5Dot3Codex is the GPT-5.3 Codex model.
	GPT5Dot3Codex = "gpt-5.3-codex"
	// GPT5Dot3Instant is the GPT-5.3 Instant model.
	GPT5Dot3Instant = "gpt-5.3-instant"
	// GPT5Dot2 is the GPT-5.2 model.
	GPT5Dot2 = "gpt-5.2"
	// GPT5Dot2Chat is the GPT-5.2 chat-latest model.
	GPT5Dot2Chat = "gpt-5.2-chat-latest"
	// GPT5Dot2Pro is the GPT-5.2 Pro model.
	GPT5Dot2Pro = "gpt-5.2-pro"
	// GPT5Dot1 is the GPT-5.1 model.
	GPT5Dot1 = "gpt-5.1"
	// GPT5 is the base GPT-5 model.
	GPT5 = "gpt-5"
	// GPT5Mini is the GPT-5 Mini model.
	GPT5Mini = "gpt-5-mini"
	// GPT5Nano is the GPT-5 Nano model.
	GPT5Nano = "gpt-5-nano"
	// GPT4Dot1 is the GPT-4.1 model.
	GPT4Dot1 = "gpt-4.1"
	// GPT4Dot1Mini is the GPT-4.1 Mini model.
	GPT4Dot1Mini = "gpt-4.1-mini"
	// GPT4Dot1Nano is the GPT-4.1 Nano model.
	GPT4Dot1Nano = "gpt-4.1-nano"
	// GPT4o is the GPT-4o model.
	GPT4o = "gpt-4o"
	// GPT4Turbo is the GPT-4 Turbo model.
	GPT4Turbo = "gpt-4-turbo"
	// GPT4 is the GPT-4 model.
	GPT4 = "gpt-4"
	// GPT3Dot5Turbo is the GPT-3.5 Turbo model.
	GPT3Dot5Turbo = "gpt-3.5-turbo"
	// GPT3Dot5Turbo16K is the GPT-3.5 Turbo 16K model.
	GPT3Dot5Turbo16K = "gpt-3.5-turbo-16k"
	// O1 is the o1 reasoning model.
	O1 = "o1"
	// O1Mini is the o1-mini reasoning model.
	O1Mini = "o1-mini"
	// O3 is the o3 reasoning model.
	O3 = "o3"
	// O3Mini is the o3-mini reasoning model.
	O3Mini = "o3-mini"
	// O3Pro is the o3-pro reasoning model.
	O3Pro = "o3-pro"
	// O4Mini is the o4-mini reasoning model.
	O4Mini = "o4-mini"

	// OpenAICompatibleURL indicates use of the default OpenAI API endpoint.
	OpenAICompatibleURL = "" // indicates to underlying client to use the default openai api
)

// IsReasoningModel returns true for OpenAI o-series reasoning models
// (o1, o3, o4 prefixes). These models do not support Temperature or
// penalty parameters.
func IsReasoningModel(model string) bool {
	for _, prefix := range []string{"o1", "o3", "o4"} {
		if model == prefix || strings.HasPrefix(model, prefix+"-") {
			return true
		}
	}
	return false
}

// SupportsReasoning returns true for models that support reasoning
// parameters (ReasoningEffort, MaxCompletionTokens). This includes
// o-series models and GPT-5.x models.
func SupportsReasoning(model string) bool {
	if IsReasoningModel(model) {
		return true
	}
	return strings.HasPrefix(model, "gpt-5")
}

// oSeriesEfforts are the effort levels supported by o-series reasoning models
// (o1, o3, o4). Per OpenAI docs these models accept low/medium/high only.
var oSeriesEfforts = map[string]bool{
	"low":    true,
	"medium": true,
	"high":   true,
}

// gpt5BaseEfforts covers GPT-5, GPT-5-mini, GPT-5-nano.
// Per OpenAI docs: minimal, low, medium, high.
var gpt5BaseEfforts = map[string]bool{
	"minimal": true,
	"low":     true,
	"medium":  true,
	"high":    true,
}

// gpt5Dot1Efforts covers GPT-5.1.
// Per OpenAI docs: none, low, medium, high.
var gpt5Dot1Efforts = map[string]bool{
	"none":   true,
	"low":    true,
	"medium": true,
	"high":   true,
}

// gpt5Dot2Efforts covers GPT-5.2, GPT-5.2-pro, GPT-5.2-chat-latest.
// Per OpenAI docs: none, low, medium, high, xhigh.
var gpt5Dot2Efforts = map[string]bool{
	"none":   true,
	"low":    true,
	"medium": true,
	"high":   true,
	"xhigh":  true,
}

// gpt5Dot3CodexEfforts covers GPT-5.3-codex.
// Per OpenAI docs: low, medium, high, xhigh.
var gpt5Dot3CodexEfforts = map[string]bool{
	"low":    true,
	"medium": true,
	"high":   true,
	"xhigh":  true,
}

// gpt5Dot4Efforts covers GPT-5.4, GPT-5.4-mini, GPT-5.4-nano.
// Per OpenAI docs: none, low, medium, high, xhigh.
var gpt5Dot4Efforts = map[string]bool{
	"none":   true,
	"low":    true,
	"medium": true,
	"high":   true,
	"xhigh":  true,
}

// gpt5Dot4ProEfforts covers GPT-5.4-pro.
// Per OpenAI docs: medium, high, xhigh.
var gpt5Dot4ProEfforts = map[string]bool{
	"medium": true,
	"high":   true,
	"xhigh":  true,
}

// supportedEfforts returns the set of supported effort levels for a given
// GPT-5.x model. Returns nil if the model is not a known GPT-5 variant
// (caller should fall back to gpt5Dot4Efforts for unknown gpt-5 prefixes).
func supportedEfforts(model string) map[string]bool {
	switch {
	case model == GPT5Dot4Pro:
		return gpt5Dot4ProEfforts
	case model == GPT5Dot5 || strings.HasPrefix(model, "gpt-5.4"):
		return gpt5Dot4Efforts
	case model == GPT5Dot3Codex || strings.HasPrefix(model, "gpt-5.3-codex"):
		return gpt5Dot3CodexEfforts
	case strings.HasPrefix(model, "gpt-5.3"):
		// gpt-5.3-instant and other 5.3 variants — use 5.2 effort set as
		// the closest documented peer.
		return gpt5Dot2Efforts
	case strings.HasPrefix(model, "gpt-5.2"):
		return gpt5Dot2Efforts
	case strings.HasPrefix(model, "gpt-5.1"):
		return gpt5Dot1Efforts
	case strings.HasPrefix(model, "gpt-5"):
		// gpt-5, gpt-5-mini, gpt-5-nano
		return gpt5BaseEfforts
	default:
		return nil
	}
}

// NormalizeEffort validates the requested effort level against the given
// model's capabilities. It returns the effort to use (empty string means
// omit the parameter entirely) and a human-readable warning when the
// requested effort is not supported by the model.
func NormalizeEffort(model, effort string) (normalized string, warning string) {
	if effort == "" {
		return "", ""
	}

	if !SupportsReasoning(model) {
		// Model doesn't support reasoning effort at all — drop silently.
		return "", ""
	}

	var supported map[string]bool
	if IsReasoningModel(model) {
		supported = oSeriesEfforts
	} else {
		supported = supportedEfforts(model)
		if supported == nil {
			// Unknown gpt-5 variant — default to the broadest set.
			supported = gpt5Dot4Efforts
		}
	}

	if supported[effort] {
		return effort, ""
	}

	return "", fmt.Sprintf(
		"Effort %q is not supported by %s; using model default instead.", effort, model)
}

// AvailableModels returns a map from model identifier -> nominal maximum
// context window (tokens). These values are collected from provider
// documentation and public release notes as of early 2026. They are a
// convenience for client-side capacity checks — always verify with the
// provider at runtime for account- or region-specific limits.
func AvailableModels() map[string]int {
	return map[string]int{
		GPT5Dot5:         1050000,
		GPT5Dot4:         1050000,
		GPT5Dot4Pro:      1050000,
		GPT5Dot4Mini:     1050000,
		GPT5Dot4Nano:     1050000,
		GPT5Dot3Codex:    400000,
		GPT5Dot3Instant:  128000,
		GPT5Dot2:         400000,
		GPT5Dot2Chat:     400000,
		GPT5Dot2Pro:      400000,
		GPT5Dot1:         400000,
		GPT5:             400000,
		GPT5Mini:         400000,
		GPT5Nano:         400000,
		GPT4Dot1:         1047576,
		GPT4Dot1Mini:     1047576,
		GPT4Dot1Nano:     1047576,
		GPT4o:            128000,
		GPT4Turbo:        128000,
		GPT4:             8192,
		GPT3Dot5Turbo:    16000,
		GPT3Dot5Turbo16K: 16000,
		O1:               200000,
		O1Mini:           128000,
		O3:               200000,
		O3Mini:           200000,
		O3Pro:            200000,
		O4Mini:           200000,
	}
}

// ModelEntries returns the static catalog of OpenAI models in this
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
			BaseURL:       OpenAICompatibleURL,
		})
	}
	return out
}
