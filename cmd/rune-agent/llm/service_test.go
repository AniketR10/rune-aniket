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

package llm

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageUnmarshalJSON(t *testing.T) {
	t.Run("new format round-trip", func(t *testing.T) {
		original := Message{
			Role:    RoleAssistant,
			Content: "hello",
			ToolCalls: []ToolCall{
				{ID: "c1", Type: ToolTypeFunction, Function: FunctionCall{Name: "read_file", Arguments: `{"path":"x"}`}},
			},
			MultiContent: []ContentPart{
				{Type: ContentPartTypeText, Text: "hello"},
			},
		}
		data, err := json.Marshal(original)
		require.NoError(t, err)

		var msg Message
		require.NoError(t, json.Unmarshal(data, &msg))
		assert.Equal(t, original.Role, msg.Role)
		assert.Equal(t, original.Content, msg.Content)
		require.Len(t, msg.ToolCalls, 1)
		assert.Equal(t, "read_file", msg.ToolCalls[0].Function.Name)
		require.Len(t, msg.MultiContent, 1)
		assert.Equal(t, "hello", msg.MultiContent[0].Text)
	})

	t.Run("old format backward compat with Metadata", func(t *testing.T) {
		// Simulate old persisted JSON: ChatCompletionMessage with Metadata containing ToolCalls.
		oldJSON := `{
			"Role": "assistant",
			"Content": "Let me read.",
			"Metadata": {
				"ToolCalls": [
					{"ID": "call_1", "Type": "function", "Function": {"Name": "read_file", "Arguments": "{\"path\":\"x\"}"}}
				]
			}
		}`

		var msg Message
		require.NoError(t, json.Unmarshal([]byte(oldJSON), &msg))
		assert.Equal(t, RoleAssistant, msg.Role)
		assert.Equal(t, "Let me read.", msg.Content)
		require.Len(t, msg.ToolCalls, 1)
		assert.Equal(t, "call_1", msg.ToolCalls[0].ID)
		assert.Equal(t, "read_file", msg.ToolCalls[0].Function.Name)
	})

	t.Run("old format backward compat with OtherContent", func(t *testing.T) {
		// Simulate old persisted JSON: ChatCompletionMessage with OtherContent.
		oldJSON := `{
			"Role": "user",
			"Content": "",
			"OtherContent": [
				{"Type": 0, "Text": "describe this"},
				{"Type": 1, "ImageURL": "http://img.png"}
			]
		}`

		var msg Message
		require.NoError(t, json.Unmarshal([]byte(oldJSON), &msg))
		require.Len(t, msg.MultiContent, 2)
		assert.Equal(t, ContentPartTypeText, msg.MultiContent[0].Type)
		assert.Equal(t, "describe this", msg.MultiContent[0].Text)
		assert.Equal(t, ContentPartTypeImageURL, msg.MultiContent[1].Type)
		assert.Equal(t, "http://img.png", msg.MultiContent[1].ImageURL)
	})

	t.Run("message without tool calls or multi-content", func(t *testing.T) {
		plainJSON := `{"Role": "user", "Content": "hello"}`

		var msg Message
		require.NoError(t, json.Unmarshal([]byte(plainJSON), &msg))
		assert.Equal(t, RoleUser, msg.Role)
		assert.Equal(t, "hello", msg.Content)
		assert.Empty(t, msg.ToolCalls)
		assert.Empty(t, msg.MultiContent)
	})

	t.Run("JSON deserialized Metadata (map[string]interface{})", func(t *testing.T) {
		// Simulate old persisted JSON with Metadata containing tool calls.
		oldJSON := `{
			"Role": "assistant",
			"Content": "",
			"Metadata": {
				"ToolCalls": [
					{"ID": "c1", "Type": "function", "Function": {"Name": "edit_file", "Arguments": "{}"}}
				]
			}
		}`

		var msg Message
		require.NoError(t, json.Unmarshal([]byte(oldJSON), &msg))
		require.Len(t, msg.ToolCalls, 1)
		assert.Equal(t, "c1", msg.ToolCalls[0].ID)
		assert.Equal(t, "edit_file", msg.ToolCalls[0].Function.Name)
	})
}
