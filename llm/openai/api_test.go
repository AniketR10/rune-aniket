// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package openai

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

func TestOpenAIMessageFromModel_JSONDeserializedToolCalls(t *testing.T) {
	// Simulate a message that was persisted and loaded back from JSON.
	original := llmapi.Message{
		Role:    llmapi.RoleAssistant,
		Content: "Let me read that file.",
		ToolCalls: []llmapi.ToolCall{
			{
				ID:   "call_123",
				Type: llmapi.ToolTypeFunction,
				Function: llmapi.FunctionCall{
					Name:      "read_file",
					Arguments: `{"path":"/tmp/test.txt"}`,
				},
			},
		},
	}

	// Round-trip through JSON to simulate persistence
	data, err := json.Marshal(original)
	require.NoError(t, err)

	var deserialized llmapi.Message
	require.NoError(t, json.Unmarshal(data, &deserialized))

	// This must not panic
	result, err := openAIMessageFromModel(deserialized)
	require.NoError(t, err)
	assert.Len(t, result.OfAssistant.ToolCalls, 1)
	assert.Equal(t, "call_123", result.OfAssistant.ToolCalls[0].OfFunction.ID)
	assert.Equal(t, "read_file", result.OfAssistant.ToolCalls[0].OfFunction.Function.Name)
	assert.Equal(t, `{"path":"/tmp/test.txt"}`, result.OfAssistant.ToolCalls[0].OfFunction.Function.Arguments)
}

func TestOpenAIMessageFromModel_NoToolCalls(t *testing.T) {
	msg := llmapi.Message{
		Role:    llmapi.RoleAssistant,
		Content: "Hello!",
	}
	result, err := openAIMessageFromModel(msg)
	require.NoError(t, err)
	assert.Empty(t, result.OfAssistant.ToolCalls)
}

func TestOpenAIMessageFromModel_WithToolCalls(t *testing.T) {
	msg := llmapi.Message{
		Role: llmapi.RoleAssistant,
		ToolCalls: []llmapi.ToolCall{
			{
				ID:   "call_456",
				Type: llmapi.ToolTypeFunction,
				Function: llmapi.FunctionCall{
					Name:      "edit_file",
					Arguments: `{"path":"/tmp/x.txt"}`,
				},
			},
		},
	}
	result, err := openAIMessageFromModel(msg)
	require.NoError(t, err)
	assert.Len(t, result.OfAssistant.ToolCalls, 1)
	assert.Equal(t, "edit_file", result.OfAssistant.ToolCalls[0].OfFunction.Function.Name)
}

func TestResponsesToolsFromModel(t *testing.T) {
	tools := []llmapi.Tool{
		{
			Type: llmapi.ToolTypeFunction,
			Function: llmapi.FunctionDefinition{
				Name:        "read_file",
				Description: "Read a file",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path":   map[string]any{"type": "string", "description": "File path"},
						"offset": map[string]any{"type": []string{"integer", "null"}, "description": "Line offset"},
					},
					"required":             []string{"path", "offset"},
					"additionalProperties": false,
				},
			},
		},
	}

	result := responsesToolsFromModel(tools)
	require.Len(t, result, 1)

	ft := result[0].OfFunction
	require.NotNil(t, ft)
	assert.Equal(t, "read_file", ft.Name)

	// Verify the function produces valid JSON with strict-compliant schema.
	data, err := json.Marshal(ft)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"additionalProperties":false`)
}
