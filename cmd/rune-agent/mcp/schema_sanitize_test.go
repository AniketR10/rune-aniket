// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package mcp

import (
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeMCPInputSchema(t *testing.T) {
	tests := []struct {
		name   string
		input  map[string]any
		assert func(t *testing.T, out map[string]any)
	}{
		{
			name: "top-level array missing items",
			input: map[string]any{
				"type": "array",
			},
			assert: func(t *testing.T, out map[string]any) {
				items, ok := out["items"].(map[string]any)
				require.True(t, ok, "items must be a map")
				assert.Empty(t, items)
			},
		},
		{
			name: "nested array under properties.foo.properties.bar",
			input: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"foo": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"bar": map[string]any{"type": "array"},
						},
					},
				},
			},
			assert: func(t *testing.T, out map[string]any) {
				bar := out["properties"].(map[string]any)["foo"].(map[string]any)["properties"].(map[string]any)["bar"].(map[string]any)
				_, ok := bar["items"].(map[string]any)
				assert.True(t, ok)
			},
		},
		{
			name: "array already has items - untouched",
			input: map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			assert: func(t *testing.T, out map[string]any) {
				items := out["items"].(map[string]any)
				assert.Equal(t, "string", items["type"])
			},
		},
		{
			name: "non-array property without items - untouched",
			input: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
			assert: func(t *testing.T, out map[string]any) {
				name := out["properties"].(map[string]any)["name"].(map[string]any)
				_, has := name["items"]
				assert.False(t, has)
			},
		},
		{
			name: "array under oneOf",
			input: map[string]any{
				"oneOf": []any{
					map[string]any{"type": "array"},
				},
			},
			assert: func(t *testing.T, out map[string]any) {
				arr := out["oneOf"].([]any)[0].(map[string]any)
				_, ok := arr["items"].(map[string]any)
				assert.True(t, ok)
			},
		},
		{
			name: "array under $defs",
			input: map[string]any{
				"$defs": map[string]any{
					"thing": map[string]any{"type": "array"},
				},
			},
			assert: func(t *testing.T, out map[string]any) {
				thing := out["$defs"].(map[string]any)["thing"].(map[string]any)
				_, ok := thing["items"].(map[string]any)
				assert.True(t, ok)
			},
		},
		{
			name: "union array|null type",
			input: map[string]any{
				"type": []any{"array", "null"},
			},
			assert: func(t *testing.T, out map[string]any) {
				_, ok := out["items"].(map[string]any)
				assert.True(t, ok)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeMCPInputSchema("server", "tool", tc.input)
			out, ok := got.(map[string]any)
			require.True(t, ok)
			tc.assert(t, out)
		})
	}
}

func TestSanitizeMCPInputSchema_Nil(t *testing.T) {
	assert.Nil(t, sanitizeMCPInputSchema("server", "tool", nil))
}

func TestSanitizeMCPInputSchema_NonMap(t *testing.T) {
	// Non-map InputSchemas (e.g. json.RawMessage, string) are returned
	// unchanged so the upstream value is preserved.
	in := "not a schema"
	out := sanitizeMCPInputSchema("server", "tool", in)
	assert.Equal(t, in, out)
}

// TestToolAdapterDefinition_SanitizesArrayMissingItems exercises the
// integration point: an MCP tool whose InputSchema has an array property
// without `items` must be patched by Definition() so it does not break
// OpenAI's strict function-schema validator.
func TestToolAdapterDefinition_SanitizesArrayMissingItems(t *testing.T) {
	mcpTool := &gomcp.Tool{
		Name:        "syntax_query",
		Description: "Query syntax",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"captures": map[string]any{"type": "array"},
			},
		},
	}
	adapter := &toolAdapter{
		serverName: "rune",
		toolName:   mcpTool.Name,
		mcpTool:    mcpTool,
	}

	def := adapter.Definition()
	params, ok := def.Function.Parameters.(map[string]any)
	require.True(t, ok)
	captures, ok := params["properties"].(map[string]any)["captures"].(map[string]any)
	require.True(t, ok)
	items, ok := captures["items"].(map[string]any)
	require.True(t, ok, "items must be injected as a map")
	assert.NotNil(t, items)
}
