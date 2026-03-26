// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2026 Unstable Build, All Rights Reserved.
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

package gemini

import "unstable.build/go-tui/cmd/rune-agent/llm/llmregistry"

// LLMProvider identifies the Gemini provider in the model registry.
const LLMProvider = "gemini"

const (
	// Gemini_3_1_Pro_Preview is the Gemini 3.1 Pro preview model.
	Gemini_3_1_Pro_Preview      = "gemini-3.1-pro-preview"
	// Gemini_3_Flash_Preview is the Gemini 3 Flash preview model.
	Gemini_3_Flash_Preview      = "gemini-3-flash-preview"
	// Gemini_3_1_FlashLite_Preview is the Gemini 3.1 Flash Lite preview model.
	Gemini_3_1_FlashLite_Preview = "gemini-3.1-flash-lite-preview"
	// Gemini_2_5_Pro is the Gemini 2.5 Pro model.
	Gemini_2_5_Pro              = "gemini-2.5-pro"
	// Gemini_2_5_Flash is the Gemini 2.5 Flash model.
	Gemini_2_5_Flash            = "gemini-2.5-flash"
	// Gemini_2_5_FlashLite is the Gemini 2.5 Flash Lite model.
	Gemini_2_5_FlashLite        = "gemini-2.5-flash-lite"
	// Gemini_2_0_Flash is the Gemini 2.0 Flash model.
	Gemini_2_0_Flash            = "gemini-2.0-flash"
	// Gemini_2_0_FlashLite is the Gemini 2.0 Flash Lite model.
	Gemini_2_0_FlashLite        = "gemini-2.0-flash-lite"

	// OpenAICompatibleURL is the OpenAI-compatible Gemini endpoint.
	OpenAICompatibleURL = "https://generativelanguage.googleapis.com/v1beta/openai/"
)

// AvailableModels returns a map from model identifier -> nominal maximum
// context window (tokens). These values are collected from provider
// documentation and public release notes as of early 2026. They are a
// convenience for client-side capacity checks — always verify with the
// provider at runtime for account- or region-specific limits.
func AvailableModels() map[string]int {
	return map[string]int{
		Gemini_3_1_Pro_Preview:       1048576,
		Gemini_3_Flash_Preview:       1048576,
		Gemini_3_1_FlashLite_Preview: 1048576,
		Gemini_2_5_Pro:               1048576,
		Gemini_2_5_Flash:             1048576,
		Gemini_2_5_FlashLite:         1048576,
		Gemini_2_0_Flash:             1048576,
		Gemini_2_0_FlashLite:         1048576,
	}
}

// RegisterModels registers all known Gemini models into the given registry.
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
