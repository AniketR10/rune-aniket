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

package claude

import (
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/go-tui/llm/anthropic"
)

// LLMProvider identifies the Claude provider in the model registry. The
// claude provider disambiguates Anthropic models authenticated with a
// Claude Code subscription (OAuth) from the api-key `anthropic` provider,
// exactly as `codex` disambiguates from `openai`.
const LLMProvider = "claude"

// ModelEntries returns the static catalog of Claude models. It re-stamps
// the Anthropic catalog with Provider "claude" so callers can target the
// subscription-authenticated path while keeping identical model slugs and
// context windows.
func ModelEntries() []llmapi.ModelEntry {
	src := anthropic.ModelEntries()
	out := make([]llmapi.ModelEntry, 0, len(src))
	for _, entry := range src {
		entry.Provider = LLMProvider
		out = append(out, entry)
	}
	return out
}

// FlagshipModel returns the provider's top model identifier. The Claude
// Code subscription path pins Opus 4.8 as its flagship rather than
// mirroring the Anthropic api-key flagship.
func FlagshipModel() string { return anthropic.ClaudeOpus4Dot8 }

// MaxOutputTokens returns the model's documented maximum output-token
// ceiling. It mirrors the Anthropic catalog since the slugs are identical.
func MaxOutputTokens(model string) int { return anthropic.MaxOutputTokens(model) }
