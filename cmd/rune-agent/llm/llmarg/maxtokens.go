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

package llmarg

import (
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/go-tui/llm/anthropic"
	"unstable.build/go-tui/llm/claude"
	"unstable.build/go-tui/llm/codex"
	"unstable.build/go-tui/llm/gemini"
	"unstable.build/go-tui/llm/openai"
)

// MaxOutputTokens returns entry's documented maximum output-token
// (API max_tokens) ceiling, or 0 when the limit is unknown — which the
// caller treats as "no client-side cap". It dispatches on the entry's
// provider to the matching catalog. Local (llamacpp) and unrecognised
// providers have no published per-model ceiling and return 0.
func MaxOutputTokens(entry llmapi.ModelEntry) int {
	switch entry.Provider {
	case anthropic.LLMProvider:
		return anthropic.MaxOutputTokens(entry.Name)
	case claude.LLMProvider:
		return claude.MaxOutputTokens(entry.Name)
	case openai.LLMProvider:
		return openai.MaxOutputTokens(entry.Name)
	case codex.LLMProvider:
		return codex.MaxOutputTokens(entry.Name)
	case gemini.LLMProvider:
		return gemini.MaxOutputTokens(entry.Name)
	default:
		return 0
	}
}

// ValidateMaxOutputTokens rejects n when it exceeds entry's documented
// maximum output-token ceiling. When the ceiling is unknown (0) any value
// passes, so a value is never wrongly rejected for an uncatalogued model.
func ValidateMaxOutputTokens(entry llmapi.ModelEntry, n int) error {
	limit := MaxOutputTokens(entry)
	if limit > 0 && n > limit {
		return fmt.Errorf(
			"%s supports at most %d max output tokens; %d is too large",
			entry.Name, limit, n)
	}
	return nil
}
