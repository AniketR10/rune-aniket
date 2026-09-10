// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package llmarg

import (
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/rune/internal/llm/anthropic"
	"unstable.build/rune/internal/llm/claude"
	"unstable.build/rune/internal/llm/codex"
	"unstable.build/rune/internal/llm/gemini"
	"unstable.build/rune/internal/llm/openai"
)

// MaxOutputTokens returns entry's documented, client-settable output-token
// ceiling. It returns 0 when the ceiling is unknown or cannot be configured.
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

// ValidateMaxOutputTokens rejects unsupported overrides and values above a
// documented ceiling. Unknown ceilings accept any value.
func ValidateMaxOutputTokens(entry llmapi.ModelEntry, n int) error {
	if entry.Provider == codex.LLMProvider {
		return fmt.Errorf("%s/%s does not support custom max output token limits",
			entry.Provider, entry.Name)
	}
	limit := MaxOutputTokens(entry)
	if limit > 0 && n > limit {
		return fmt.Errorf(
			"%s supports at most %d max output tokens; %d is too large",
			entry.Name, limit, n)
	}
	return nil
}
