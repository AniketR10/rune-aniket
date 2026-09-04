// Copyright (C) 2017-2026 Unstable Build, LLC
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

package anthropic

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

func TestAnthropicParamsClaudeCodeSpoofPrependsSystemBlocks(t *testing.T) {
	request := llmapi.Request{
		Messages: []llmapi.Message{
			{Role: llmapi.RoleSystem, Content: "You are the rune agent."},
			{Role: llmapi.RoleUser, Content: "Hi"},
		},
	}

	params := anthropicParamsFromRequest("claude-opus-4-8", request, Config{ClaudeCodeSpoof: true})

	require.GreaterOrEqual(t, len(params.System), 4)
	assert.True(t, strings.HasPrefix(params.System[0].Text, claudeCodeBillingHeaderPrefix),
		"system[0] must be the billing header, got %q", params.System[0].Text)
	assert.Equal(t, claudeCodeAgentIdentifier, params.System[1].Text)
	assert.Equal(t, claudeCodeStaticPrompt, params.System[2].Text)
	assert.Equal(t, "You are the rune agent.", params.System[3].Text)
}

func TestAnthropicParamsNoSpoofLeavesSystemUnchanged(t *testing.T) {
	request := llmapi.Request{
		Messages: []llmapi.Message{
			{Role: llmapi.RoleSystem, Content: "You are the rune agent."},
			{Role: llmapi.RoleUser, Content: "Hi"},
		},
	}

	params := anthropicParamsFromRequest("claude-opus-4-8", request, Config{})

	require.Len(t, params.System, 1)
	assert.Equal(t, "You are the rune agent.", params.System[0].Text)
}

func TestAnthropicParamsClaudeCodeSpoofNoSystemMessages(t *testing.T) {
	request := llmapi.Request{
		Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "Hi"}},
	}

	params := anthropicParamsFromRequest("claude-opus-4-8", request, Config{ClaudeCodeSpoof: true})

	require.Len(t, params.System, 3)
	assert.True(t, strings.HasPrefix(params.System[0].Text, claudeCodeBillingHeaderPrefix))
	assert.Equal(t, claudeCodeAgentIdentifier, params.System[1].Text)
	assert.Equal(t, claudeCodeStaticPrompt, params.System[2].Text)
}
