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

package codex

import (
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/go-tui/llm/openai"
)

// LLMProvider identifies the Codex provider in the model registry.
const LLMProvider = "codex"

const (
	// GPT5Dot5 is the frontier Codex model.
	GPT5Dot5 = "gpt-5.5"
	// GPT5Dot4 is the strong everyday Codex coding model.
	GPT5Dot4 = "gpt-5.4"
	// GPT5Dot4Mini is the small Codex coding model.
	GPT5Dot4Mini = "gpt-5.4-mini"
	// GPT5Dot3Codex is the Codex-optimized GPT-5.3 model.
	GPT5Dot3Codex = "gpt-5.3-codex"
	// GPT5Dot2 is the older professional-work Codex model.
	GPT5Dot2 = "gpt-5.2"
	// CodexAutoReview is the Codex automatic review model.
	CodexAutoReview = "codex-auto-review"

	// OpenAICompatibleURL is the ChatGPT Codex backend OpenAI-compatible API.
	OpenAICompatibleURL = "https://chatgpt.com/backend-api/codex"
)

// AvailableModels returns Codex models exposed by the Codex backend.
func AvailableModels() map[string]int {
	// Context windows mirror codex-rs/models-manager/models.json. For
	// `gpt-5.4` and `codex-auto-review` the bundled `max_context_window`
	// is 1M (with a 272k default plan budget); the other Codex slugs
	// have a 272k upstream ceiling.
	return map[string]int{
		GPT5Dot5:        272000,
		GPT5Dot4:        1000000,
		GPT5Dot4Mini:    272000,
		GPT5Dot3Codex:   272000,
		GPT5Dot2:        272000,
		CodexAutoReview: 1000000,
	}
}

// ModelEntries returns the static catalog of Codex models in this
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

// FlagshipModel returns the provider's top model identifier. It is
// deterministic, unlike iterating ModelEntries() whose order is
// map-random.
func FlagshipModel() string { return GPT5Dot5 }

// UpstreamModelName returns the Codex backend model slug for a given
// catalog name. The rune-side codex catalog stores the upstream slug
// directly (the legacy `codex/` registry prefix was only ever needed
// when codex and openai models shared a flat namespace keyed by Name);
// this helper is kept as the identity function so callers do not need
// to know whether the rune-agent transitional namespace is in use.
func UpstreamModelName(model string) string { return model }

// MaxOutputTokens returns the model's documented maximum output-token
// ceiling, or 0 when the limit is unknown. Codex slugs are a subset of
// the GPT-5 family, so the lookup delegates to the OpenAI catalog keyed
// by the upstream model slug.
func MaxOutputTokens(model string) int {
	return openai.MaxOutputTokens(UpstreamModelName(model))
}
