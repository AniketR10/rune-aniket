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
