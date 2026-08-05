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

package mcp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/rune-agent/agent"
)

const everythingServerFixture = "testdata/server-everything"

type everythingServerTest struct {
	name             string
	tool             string
	arguments        string
	wantContent      string
	wantJSON         map[string]any
	wantPropertyType map[string]string
}

var everythingServerTests = []everythingServerTest{
	{
		name:             "echoes text",
		tool:             "echo",
		arguments:        `{"message":"hello from Rune"}`,
		wantContent:      "Echo: hello from Rune",
		wantPropertyType: map[string]string{"message": "string"},
	},
	{
		name:             "accepts numeric arguments",
		tool:             "get-sum",
		arguments:        `{"a":20,"b":22}`,
		wantContent:      "The sum of 20 and 22 is 42.",
		wantPropertyType: map[string]string{"a": "number", "b": "number"},
	},
	{
		name:        "keeps text around image content",
		tool:        "get-tiny-image",
		arguments:   `{}`,
		wantContent: "Here's the image you requested:\nThe image above is the MCP logo.",
	},
	{
		name:        "keeps text alongside resource links",
		tool:        "get-resource-links",
		arguments:   `{"count":2}`,
		wantContent: "Here are 2 resource links to resources available in this server:",
	},
	{
		name:      "returns backward-compatible structured content",
		tool:      "get-structured-content",
		arguments: `{"location":"New York"}`,
		wantJSON: map[string]any{
			"temperature": float64(33),
			"conditions":  "Cloudy",
			"humidity":    float64(82),
		},
		wantPropertyType: map[string]string{"location": "string"},
	},
}

func TestEverythingServerE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real MCP server in short mode")
	}

	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node not found; install Node.js to run the MCP e2e suite")
	}
	serverEntry := installEverythingServer(t)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()

	m := NewManager(newLocalExec(), t.TempDir())
	t.Cleanup(func() { require.NoError(t, m.Close()) })

	tools, err := m.ConnectServer(ctx, "everything", ServerConfig{
		Type:    "stdio",
		Command: node,
		Args:    []string{serverEntry, "stdio"},
	})
	require.NoError(t, err)
	require.NotEmpty(t, tools)

	toolsByName := indexTools(tools)
	server := requireEverythingServer(t, m, len(tools))

	for _, tt := range everythingServerTests {
		t.Run(tt.name, func(t *testing.T) {
			toolName := "everything_" + tt.tool
			tool := toolsByName[toolName]
			require.NotNil(t, tool, "%s was not advertised; got %v", toolName, server.ToolNames)

			if tt.wantPropertyType != nil {
				requireSchemaProperties(t, tool, tt.wantPropertyType)
			}

			callCtx, callCancel := context.WithTimeout(ctx, 10*time.Second)
			defer callCancel()
			result := tool.Execute(callCtx, tt.arguments)
			require.False(t, result.IsError, result.Content)

			if tt.wantJSON != nil {
				var got map[string]any
				require.NoError(t, json.Unmarshal([]byte(result.Content), &got))
				assert.Equal(t, tt.wantJSON, got)
			} else {
				assert.Equal(t, tt.wantContent, result.Content)
			}
		})
	}
}

func installEverythingServer(t *testing.T) string {
	t.Helper()

	npm, err := exec.LookPath("npm")
	if err != nil {
		t.Skip("npm not found; install Node.js to run the MCP e2e suite")
	}

	dir := t.TempDir()
	for _, name := range []string{"package.json", "package-lock.json"} {
		data, err := os.ReadFile(filepath.Join(everythingServerFixture, name))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o600))
	}

	cmd := exec.Command(
		npm, "ci", "--ignore-scripts", "--no-bin-links", "--no-audit", "--no-fund",
	)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "install integrity-locked MCP fixture: %s", output)

	entry := filepath.Join(
		dir, "node_modules", "@modelcontextprotocol", "server-everything", "dist", "index.js",
	)
	info, err := os.Stat(entry)
	require.NoError(t, err)
	require.False(t, info.IsDir(), "MCP server entrypoint %q is a directory", entry)
	return entry
}

func indexTools(tools []agent.Tool) map[string]agent.Tool {
	indexed := make(map[string]agent.Tool, len(tools))
	for _, tool := range tools {
		indexed[tool.Definition().Function.Name] = tool
	}
	return indexed
}

func requireEverythingServer(t *testing.T, m *Manager, toolCount int) *ServerInfo {
	t.Helper()

	servers := m.Servers()
	require.Len(t, servers, 1)
	server := servers[0]
	assert.Equal(t, "everything", server.Name)
	assert.Equal(t, StatusConnected, server.Status)
	assert.Equal(t, toolCount, server.ToolCount)
	assert.NotZero(t, server.ConnectedAt)
	return server
}

func requireSchemaProperties(
	t *testing.T, tool agent.Tool, want map[string]string,
) {
	t.Helper()

	schema, ok := tool.Definition().Function.Parameters.(map[string]any)
	require.True(t, ok, "tool parameters have type %T", tool.Definition().Function.Parameters)
	assert.Equal(t, "object", schema["type"])

	properties, ok := schema["properties"].(map[string]any)
	require.True(t, ok, "schema properties have type %T", schema["properties"])
	for name, wantType := range want {
		property, ok := properties[name].(map[string]any)
		require.True(t, ok, "schema property %q has type %T", name, properties[name])
		assert.Equal(t, wantType, property["type"], "schema property %q", name)
	}
}
