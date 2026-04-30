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

package codex

import "unstable.build/go-tui/cmd/rune-agent/llm/llmregistry"

// LLMProvider identifies the Codex provider in the model registry.
const LLMProvider = "codex"

const (
	modelPrefix = "codex/"

	// GPT5Dot5 is the frontier Codex model.
	GPT5Dot5 = modelPrefix + "gpt-5.5"
	// GPT5Dot4 is the strong everyday Codex coding model.
	GPT5Dot4 = modelPrefix + "gpt-5.4"
	// GPT5Dot4Mini is the small Codex coding model.
	GPT5Dot4Mini = modelPrefix + "gpt-5.4-mini"
	// GPT5Dot3Codex is the Codex-optimized GPT-5.3 model.
	GPT5Dot3Codex = modelPrefix + "gpt-5.3-codex"
	// GPT5Dot2 is the older professional-work Codex model.
	GPT5Dot2 = modelPrefix + "gpt-5.2"
	// CodexAutoReview is the Codex automatic review model.
	CodexAutoReview = modelPrefix + "codex-auto-review"

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

var upstreamModelNames = map[string]string{
	GPT5Dot5:        "gpt-5.5",
	GPT5Dot4:        "gpt-5.4",
	GPT5Dot4Mini:    "gpt-5.4-mini",
	GPT5Dot3Codex:   "gpt-5.3-codex",
	GPT5Dot2:        "gpt-5.2",
	CodexAutoReview: "codex-auto-review",
}

// RegisterModels registers all known Codex models into the given registry.
func RegisterModels(r llmregistry.MutableRegistry) {
	for name, ctxWindow := range AvailableModels() {
		r.Register(llmregistry.ModelEntry{
			Name:          name,
			Provider:      LLMProvider,
			ContextWindow: ctxWindow,
			BaseURL:       OpenAICompatibleURL,
		})
	}
}

// UpstreamModelName maps the registry model name to the Codex backend model slug.
func UpstreamModelName(model string) string {
	if upstream, ok := upstreamModelNames[model]; ok {
		return upstream
	}
	return model
}

// UpstreamAvailableModels returns the Codex backend model slugs accepted by the OpenAI client.
func UpstreamAvailableModels() map[string]int {
	models := make(map[string]int, len(upstreamModelNames))
	for name, ctxWindow := range AvailableModels() {
		models[UpstreamModelName(name)] = ctxWindow
	}
	return models
}
