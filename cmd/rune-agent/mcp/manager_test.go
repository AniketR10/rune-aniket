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
	"context"
	"errors"
	"sync"
	"testing"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testServer creates an MCP server with n tools and returns a transport
// factory that connects to it via in-memory transports.
func testServer(t *testing.T, toolNames ...string) (*gomcp.Server, TransportFactory) {
	t.Helper()

	server := gomcp.NewServer(
		&gomcp.Implementation{Name: "test-server", Version: "v1.0.0"}, nil,
	)
	for _, name := range toolNames {
		gomcp.AddTool(server, &gomcp.Tool{
			Name:        name,
			Description: "tool " + name,
		}, func(_ context.Context, _ *gomcp.CallToolRequest, _ any) (*gomcp.CallToolResult, any, error) {
			return &gomcp.CallToolResult{
				Content: []gomcp.Content{
					&gomcp.TextContent{Text: "ok from " + name},
				},
			}, nil, nil
		})
	}

	var mu sync.Mutex
	var sessions []*gomcp.ServerSession

	factory := func(_ string, _ ServerConfig) (gomcp.Transport, error) {
		st, ct := gomcp.NewInMemoryTransports()
		ss, err := server.Connect(context.Background(), st, nil)
		if err != nil {
			return nil, err
		}
		mu.Lock()
		sessions = append(sessions, ss)
		mu.Unlock()
		return ct, nil
	}

	t.Cleanup(func() {
		mu.Lock()
		defer mu.Unlock()
		for _, ss := range sessions {
			_ = ss.Wait()
		}
	})

	return server, factory
}

func TestManagerConnect(t *testing.T) {
	_, factory := testServer(t, "tool_a", "tool_b")

	m := NewManagerWithTransport(factory)

	cfg := Config{
		MCPServers: map[string]ServerConfig{
			"srv": {Type: "stdio", Command: "dummy"},
		},
	}

	tools := m.Connect(context.Background(), cfg)
	require.Len(t, tools, 2)

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Definition().Function.Name] = true
	}
	assert.True(t, names["srv_tool_a"])
	assert.True(t, names["srv_tool_b"])

	require.NoError(t, m.Close())
}

func TestManagerConnectMultipleServers(t *testing.T) {
	_, factoryA := testServer(t, "alpha")
	_, factoryB := testServer(t, "beta")

	// Track which server name maps to which factory.
	factories := map[string]TransportFactory{
		"a": factoryA,
		"b": factoryB,
	}

	m := NewManagerWithTransport(func(name string, cfg ServerConfig) (gomcp.Transport, error) {
		return factories[name](name, cfg)
	})

	cfg := Config{
		MCPServers: map[string]ServerConfig{
			"a": {Type: "stdio", Command: "a-cmd"},
			"b": {Type: "stdio", Command: "b-cmd"},
		},
	}

	tools := m.Connect(context.Background(), cfg)
	require.Len(t, tools, 2)

	names := make(map[string]bool)
	for _, tool := range tools {
		names[tool.Definition().Function.Name] = true
	}
	assert.True(t, names["a_alpha"])
	assert.True(t, names["b_beta"])

	require.NoError(t, m.Close())
}

func TestManagerConnectSkipsFailedTransport(t *testing.T) {
	_, goodFactory := testServer(t, "ok_tool")

	m := NewManagerWithTransport(func(name string, cfg ServerConfig) (gomcp.Transport, error) {
		if name == "bad" {
			return nil, errors.New("transport creation failed")
		}
		return goodFactory(name, cfg)
	})

	cfg := Config{
		MCPServers: map[string]ServerConfig{
			"bad":  {Type: "stdio", Command: "bad-cmd"},
			"good": {Type: "stdio", Command: "good-cmd"},
		},
	}

	tools := m.Connect(context.Background(), cfg)
	// Only the good server's tool should be present.
	require.Len(t, tools, 1)
	assert.Equal(t, "good_ok_tool", tools[0].Definition().Function.Name)

	require.NoError(t, m.Close())
}

func TestManagerConnectEmptyConfig(t *testing.T) {
	m := NewManagerWithTransport(nil)
	tools := m.Connect(context.Background(), Config{})
	assert.Empty(t, tools)
	require.NoError(t, m.Close())
}

func TestManagerCloseWithoutConnect(t *testing.T) {
	m := NewManagerWithTransport(nil)
	require.NoError(t, m.Close())
}

func TestManagerServersConnected(t *testing.T) {
	_, factory := testServer(t, "tool_a", "tool_b")

	m := NewManagerWithTransport(factory)

	cfg := Config{
		MCPServers: map[string]ServerConfig{
			"srv": {Type: "stdio", Command: "my-cmd"},
		},
	}
	m.Connect(context.Background(), cfg)

	servers := m.Servers()
	require.Len(t, servers, 1)
	assert.Equal(t, "srv", servers[0].Name)
	assert.Equal(t, "my-cmd", servers[0].Command)
	assert.Equal(t, StatusConnected, servers[0].Status)
	assert.Equal(t, 2, servers[0].ToolCount)
	assert.Equal(t, []string{"tool_a", "tool_b"}, servers[0].ToolNames)
	assert.False(t, servers[0].ConnectedAt.IsZero())

	require.NoError(t, m.Close())
}

func TestManagerServersError(t *testing.T) {
	m := NewManagerWithTransport(func(_ string, _ ServerConfig) (gomcp.Transport, error) {
		return nil, errors.New("bad transport")
	})

	cfg := Config{
		MCPServers: map[string]ServerConfig{
			"broken": {Type: "stdio", Command: "nope"},
		},
	}
	m.Connect(context.Background(), cfg)

	servers := m.Servers()
	require.Len(t, servers, 1)
	assert.Equal(t, "broken", servers[0].Name)
	assert.Equal(t, StatusError, servers[0].Status)
	assert.Contains(t, servers[0].Error, "bad transport")
	assert.Equal(t, 0, servers[0].ToolCount)

	require.NoError(t, m.Close())
}

func TestManagerServersSortedByName(t *testing.T) {
	_, factoryA := testServer(t, "a1")
	_, factoryB := testServer(t, "b1")

	factories := map[string]TransportFactory{"z": factoryA, "a": factoryB}
	m := NewManagerWithTransport(func(name string, cfg ServerConfig) (gomcp.Transport, error) {
		return factories[name](name, cfg)
	})

	cfg := Config{
		MCPServers: map[string]ServerConfig{
			"z": {Type: "stdio", Command: "z-cmd"},
			"a": {Type: "stdio", Command: "a-cmd"},
		},
	}
	m.Connect(context.Background(), cfg)

	servers := m.Servers()
	require.Len(t, servers, 2)
	assert.Equal(t, "a", servers[0].Name)
	assert.Equal(t, "z", servers[1].Name)

	require.NoError(t, m.Close())
}

func TestManagerCloseDisconnectsServers(t *testing.T) {
	_, factory := testServer(t, "tool_a")

	m := NewManagerWithTransport(factory)

	cfg := Config{
		MCPServers: map[string]ServerConfig{
			"srv": {Type: "stdio", Command: "cmd"},
		},
	}
	m.Connect(context.Background(), cfg)

	servers := m.Servers()
	require.Len(t, servers, 1)
	assert.Equal(t, StatusConnected, servers[0].Status)

	require.NoError(t, m.Close())

	servers = m.Servers()
	require.Len(t, servers, 1)
	assert.Equal(t, StatusDisconnected, servers[0].Status)
}

func TestManagerServersEmpty(t *testing.T) {
	m := NewManagerWithTransport(nil)
	assert.Empty(t, m.Servers())
}
