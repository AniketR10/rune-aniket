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

package gemini

import (
	"fmt"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

// LLMProvider identifies the Gemini provider in the model registry.
const LLMProvider = "gemini"

const (
	// Gemini_3_1_Pro_Preview is the Gemini 3.1 Pro preview model.
	Gemini_3_1_Pro_Preview = "gemini-3.1-pro-preview"
	// Gemini_3_Flash_Preview is the Gemini 3 Flash preview model.
	Gemini_3_Flash_Preview = "gemini-3-flash-preview"
	// Gemini_3_1_FlashLite_Preview is the Gemini 3.1 Flash Lite preview model.
	Gemini_3_1_FlashLite_Preview = "gemini-3.1-flash-lite-preview"
	// Gemini_2_5_Pro is the Gemini 2.5 Pro model.
	Gemini_2_5_Pro = "gemini-2.5-pro"
	// Gemini_2_5_Flash is the Gemini 2.5 Flash model.
	Gemini_2_5_Flash = "gemini-2.5-flash"
	// Gemini_2_5_FlashLite is the Gemini 2.5 Flash Lite model.
	Gemini_2_5_FlashLite = "gemini-2.5-flash-lite"
	// Gemini_2_0_Flash is the Gemini 2.0 Flash model.
	Gemini_2_0_Flash = "gemini-2.0-flash"
	// Gemini_2_0_FlashLite is the Gemini 2.0 Flash Lite model.
	Gemini_2_0_FlashLite = "gemini-2.0-flash-lite"
)

// VerificationModel is the model used to probe a key during
// VerifyProviderKey. Flash is chosen because it is cheap and broadly
// available across accounts.
const VerificationModel = Gemini_2_5_Flash

// AvailableModels returns a map from model identifier -> nominal maximum
// context window (tokens). This static catalog is the offline fallback
// used when the live Gemini listing cannot be queried — for example at
// bootstrap, before a valid API key has been entered, since the
// generativelanguage models endpoint rejects requests without a working
// key. Values are from provider documentation as of early 2026; always
// verify with the provider at runtime for account-specific limits.
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

// ModelEntries returns the static catalog of Gemini models. It is the
// offline fallback for the live listing (see client.Models) and the
// bootstrap catalog shown before a key is verified.
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
func FlagshipModel() string { return Gemini_3_1_Pro_Preview }

// supportedEfforts are the reasoning-effort levels that map onto a Gemini
// thinking level. "none" and "xhigh"/"max" have no Gemini equivalent.
var supportedEfforts = map[string]bool{
	"minimal": true,
	"low":     true,
	"medium":  true,
	"high":    true,
}

// defaultGemini3Effort is the thinking level applied to Gemini 3 models when
// no usable effort is requested. Gemini 3 returns (and on the next turn
// requires) a thought_signature on function-call parts, so a thinking config
// must always be sent — never omitted — to keep signatures flowing so long
// tool-calling conversations do not degrade into empty completions. Medium is
// used as the default because the minimal level is not accepted by every
// Gemini 3 model (Pro rejects it), whereas medium is broadly supported.
const defaultGemini3Effort = "medium"

// isGemini3 reports whether the model is a Gemini 3.x model, which mandates
// thought signatures during function calling.
func isGemini3(model string) bool {
	return strings.HasPrefix(model, "gemini-3")
}

// NormalizeEffort validates the requested effort against Gemini's thinking
// levels. It returns the effort to use (empty means omit the thinking config)
// and a human-readable warning when the requested effort is unsupported.
func NormalizeEffort(model, effort string) (normalized, warning string) {
	if effort == "" || effort == "none" {
		if isGemini3(model) {
			return defaultGemini3Effort, ""
		}
		return "", ""
	}
	if supportedEfforts[effort] {
		return effort, ""
	}
	// Unsupported effort: warn, then fall back. Gemini 3 still needs a thinking
	// config, so it falls back to the default rather than omitting it.
	warning = fmt.Sprintf(
		"Effort %q is not supported by %s; using model default instead.", effort, model)
	if isGemini3(model) {
		return defaultGemini3Effort, warning
	}
	return "", warning
}
