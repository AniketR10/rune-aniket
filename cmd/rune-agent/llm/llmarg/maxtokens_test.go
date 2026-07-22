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

package llmarg_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"

	"unstable.build/go-tui/cmd/rune-agent/llm/llmarg"
	"unstable.build/go-tui/llm/anthropic"
	"unstable.build/go-tui/llm/claude"
	"unstable.build/go-tui/llm/codex"
	"unstable.build/go-tui/llm/gemini"
	"unstable.build/go-tui/llm/openai"
)

func TestMaxOutputTokensDispatch(t *testing.T) {
	tests := []struct {
		name  string
		entry llmapi.ModelEntry
		want  int
	}{
		{"anthropic", llmapi.ModelEntry{Provider: anthropic.LLMProvider, Name: anthropic.ClaudeFable5}, 128000},
		{"claude", llmapi.ModelEntry{Provider: claude.LLMProvider, Name: anthropic.ClaudeFable5}, 128000},
		{"openai", llmapi.ModelEntry{Provider: openai.LLMProvider, Name: openai.GPT5Dot5}, 128000},
		{"codex", llmapi.ModelEntry{Provider: codex.LLMProvider, Name: codex.GPT5Dot4}, 0},
		{"gemini", llmapi.ModelEntry{Provider: gemini.LLMProvider, Name: gemini.Gemini_2_5_Flash}, 65536},
		{"gemini 3.6 flash", llmapi.ModelEntry{Provider: gemini.LLMProvider, Name: gemini.Gemini_3_6_Flash}, 65536},
		{"gemini 3.5 flash-lite", llmapi.ModelEntry{Provider: gemini.LLMProvider, Name: gemini.Gemini_3_5_FlashLite}, 65536},
		{"local", llmapi.ModelEntry{Provider: "llamacpp", Name: "anything"}, 0},
		{"unknown provider", llmapi.ModelEntry{Provider: "mystery", Name: "x"}, 0},
		{"unknown model", llmapi.ModelEntry{Provider: anthropic.LLMProvider, Name: "unknown"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, llmarg.MaxOutputTokens(tt.entry))
		})
	}
}

func TestValidateMaxOutputTokens(t *testing.T) {
	known := llmapi.ModelEntry{Provider: anthropic.LLMProvider, Name: anthropic.ClaudeFable5}

	require.NoError(t, llmarg.ValidateMaxOutputTokens(known, 128000),
		"a value at the limit must pass")
	require.NoError(t, llmarg.ValidateMaxOutputTokens(known, 1000),
		"a value under the limit must pass")

	err := llmarg.ValidateMaxOutputTokens(known, 200000)
	require.Error(t, err)
	assert.Equal(t,
		"claude-fable-5 supports at most 128000 max output tokens; 200000 is too large",
		err.Error())

	noCap := llmapi.ModelEntry{Provider: "llamacpp", Name: "local"}
	require.NoError(t, llmarg.ValidateMaxOutputTokens(noCap, 1<<30),
		"unknown ceiling must never reject")

	unknownModel := llmapi.ModelEntry{Provider: anthropic.LLMProvider, Name: "unknown"}
	require.NoError(t, llmarg.ValidateMaxOutputTokens(unknownModel, 1<<30),
		"unknown model ceiling (0) must never reject")

	codexModel := llmapi.ModelEntry{Provider: codex.LLMProvider, Name: codex.GPT5Dot6Sol}
	err = llmarg.ValidateMaxOutputTokens(codexModel, 4096)
	require.EqualError(t, err,
		"codex/gpt-5.6-sol does not support custom max output token limits")
}
