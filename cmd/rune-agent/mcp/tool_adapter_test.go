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

package mcp

import (
	"context"
	"errors"
	"testing"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

type greetArgs struct {
	Name string `json:"name"`
}

func setupTestSession(t *testing.T) *gomcp.ClientSession {
	t.Helper()
	ctx := context.Background()

	server := gomcp.NewServer(
		&gomcp.Implementation{Name: "test-server", Version: "v1.0.0"}, nil,
	)
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "greet",
		Description: "Say hello",
	}, func(_ context.Context, _ *gomcp.CallToolRequest, args greetArgs) (*gomcp.CallToolResult, any, error) {
		return &gomcp.CallToolResult{
			Content: []gomcp.Content{
				&gomcp.TextContent{Text: "Hello " + args.Name},
			},
		}, nil, nil
	})
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "fail",
		Description: "Always fails",
	}, func(_ context.Context, _ *gomcp.CallToolRequest, _ any) (*gomcp.CallToolResult, any, error) {
		return nil, nil, errors.New("something broke")
	})

	st, ct := gomcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ss.Wait() })

	client := gomcp.NewClient(
		&gomcp.Implementation{Name: "test-client", Version: "v1.0.0"}, nil,
	)
	session, err := client.Connect(ctx, ct, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })

	return session
}

func TestToolAdapterDefinition(t *testing.T) {
	session := setupTestSession(t)

	result, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	var greetTool *gomcp.Tool
	for _, tool := range result.Tools {
		if tool.Name == "greet" {
			greetTool = tool
			break
		}
	}
	require.NotNil(t, greetTool, "greet tool not found")

	adapter := &toolAdapter{
		serverName: "myserver",
		toolName:   greetTool.Name,
		mcpTool:    greetTool,
		session:    session,
	}

	def := adapter.Definition()
	assert.Equal(t, llmapi.ToolTypeFunction, def.Type)
	assert.Equal(t, "myserver_greet", def.Function.Name)
	assert.Equal(t, "Say hello", def.Function.Description)
	assert.NotNil(t, def.Function.Parameters)
}

func TestToolAdapterExecute(t *testing.T) {
	session := setupTestSession(t)

	adapter := &toolAdapter{
		serverName: "myserver",
		toolName:   "greet",
		mcpTool:    &gomcp.Tool{Name: "greet"},
		session:    session,
	}

	result := adapter.Execute(context.Background(), `{"name":"Alice"}`)
	assert.False(t, result.IsError)
	assert.Equal(t, "Hello Alice", result.Content)
}

func TestToolAdapterExecuteInvalidJSON(t *testing.T) {
	session := setupTestSession(t)

	adapter := &toolAdapter{
		serverName: "myserver",
		toolName:   "greet",
		mcpTool:    &gomcp.Tool{Name: "greet"},
		session:    session,
	}

	result := adapter.Execute(context.Background(), `{invalid`)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "invalid arguments")
}

func TestToolAdapterExecuteServerError(t *testing.T) {
	session := setupTestSession(t)

	adapter := &toolAdapter{
		serverName: "myserver",
		toolName:   "fail",
		mcpTool:    &gomcp.Tool{Name: "fail"},
		session:    session,
	}

	result := adapter.Execute(context.Background(), `{}`)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "something broke")
}

func TestToolAdapterSummary(t *testing.T) {
	adapter := &toolAdapter{
		serverName: "myserver",
		toolName:   "greet",
	}
	assert.Equal(t, "", adapter.Summary(`{"name":"Alice"}`))
}

func TestFlattenContent(t *testing.T) {
	tests := []struct {
		name     string
		contents []gomcp.Content
		want     string
	}{
		{
			name:     "empty",
			contents: nil,
			want:     "",
		},
		{
			name: "single text",
			contents: []gomcp.Content{
				&gomcp.TextContent{Text: "hello"},
			},
			want: "hello",
		},
		{
			name: "multiple text",
			contents: []gomcp.Content{
				&gomcp.TextContent{Text: "line1"},
				&gomcp.TextContent{Text: "line2"},
			},
			want: "line1\nline2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := flattenContent(tt.contents)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestToolStatsRecordSuccess(t *testing.T) {
	session := setupTestSession(t)
	stats := &ToolStats{}

	adapter := &toolAdapter{
		serverName: "myserver",
		toolName:   "greet",
		mcpTool:    &gomcp.Tool{Name: "greet"},
		session:    session,
		stats:      stats,
	}

	result := adapter.Execute(context.Background(), `{"name":"Alice"}`)
	assert.False(t, result.IsError)

	assert.Equal(t, int64(1), stats.CallCount())
	assert.Equal(t, int64(0), stats.ErrorCount())
	assert.Greater(t, int64(stats.TotalDuration()), int64(0))
	assert.False(t, stats.LastCall().IsZero())
}

func TestToolStatsRecordError(t *testing.T) {
	session := setupTestSession(t)
	stats := &ToolStats{}

	adapter := &toolAdapter{
		serverName: "myserver",
		toolName:   "fail",
		mcpTool:    &gomcp.Tool{Name: "fail"},
		session:    session,
		stats:      stats,
	}

	result := adapter.Execute(context.Background(), `{}`)
	assert.True(t, result.IsError)

	assert.Equal(t, int64(1), stats.CallCount())
	assert.Equal(t, int64(1), stats.ErrorCount())
}

func TestToolStatsRecordInvalidJSON(t *testing.T) {
	session := setupTestSession(t)
	stats := &ToolStats{}

	adapter := &toolAdapter{
		serverName: "myserver",
		toolName:   "greet",
		mcpTool:    &gomcp.Tool{Name: "greet"},
		session:    session,
		stats:      stats,
	}

	result := adapter.Execute(context.Background(), `{invalid`)
	assert.True(t, result.IsError)

	assert.Equal(t, int64(1), stats.CallCount())
	assert.Equal(t, int64(1), stats.ErrorCount())
}

func TestToolStatsMultipleCalls(t *testing.T) {
	session := setupTestSession(t)
	stats := &ToolStats{}

	adapter := &toolAdapter{
		serverName: "myserver",
		toolName:   "greet",
		mcpTool:    &gomcp.Tool{Name: "greet"},
		session:    session,
		stats:      stats,
	}

	adapter.Execute(context.Background(), `{"name":"A"}`)
	adapter.Execute(context.Background(), `{"name":"B"}`)
	adapter.Execute(context.Background(), `{"name":"C"}`)

	assert.Equal(t, int64(3), stats.CallCount())
	assert.Equal(t, int64(0), stats.ErrorCount())
}

func TestToolStatsLastCallZeroBeforeUse(t *testing.T) {
	stats := &ToolStats{}
	assert.True(t, stats.LastCall().IsZero())
	assert.Equal(t, time.Duration(0), stats.TotalDuration())
}

func TestServerInfoToolStats(t *testing.T) {
	_, factory := testServer(t, "tool_a", "tool_b")

	m := NewManagerWithTransport(factory)

	cfg := Config{
		MCPServers: map[string]ServerConfig{
			"srv": {Type: "stdio", Command: "dummy"},
		},
	}
	tools := m.Connect(context.Background(), cfg)

	servers := m.Servers()
	require.Len(t, servers, 1)

	// ToolStats should be accessible for each tool.
	statsA := servers[0].ToolStats("tool_a")
	require.NotNil(t, statsA)
	assert.Equal(t, int64(0), statsA.CallCount())

	// Execute and check stats are updated.
	for _, tool := range tools {
		if tool.Definition().Function.Name == "srv_tool_a" {
			tool.Execute(context.Background(), `{}`)
			break
		}
	}
	assert.Equal(t, int64(1), statsA.CallCount())

	// Non-existent tool returns nil.
	assert.Nil(t, servers[0].ToolStats("nonexistent"))

	require.NoError(t, m.Close())
}
