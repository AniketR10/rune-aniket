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
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"unstable.build/go-tui/cmd/rune-agent/agent"
)

const connectTimeout = 30 * time.Second

// ServerStatus represents the current connection state of an MCP server.
type ServerStatus string

const (
	// StatusConnected indicates that the MCP server is connected.
	StatusConnected    ServerStatus = "connected"
	// StatusError indicates that the MCP server encountered an error.
	StatusError        ServerStatus = "error"
	// StatusDisconnected indicates that the MCP server is disconnected.
	StatusDisconnected ServerStatus = "disconnected"
)

// ServerInfo holds a snapshot of a connected MCP server's state.
type ServerInfo struct {
	Name        string
	Command     string
	Status      ServerStatus
	Error       string
	ConnectedAt time.Time
	ToolCount   int
	ToolNames   []string
	toolStats   map[string]*ToolStats
}

// ToolStats returns the execution stats for the named tool, or nil.
func (si *ServerInfo) ToolStats(name string) *ToolStats {
	if si.toolStats == nil {
		return nil
	}
	return si.toolStats[name]
}

// serverEntry tracks a single MCP server connection.
type serverEntry struct {
	name    string
	config  ServerConfig
	session *gomcp.ClientSession
	info    *ServerInfo
}

// TransportFactory creates a transport for a given server configuration.
type TransportFactory func(name string, cfg ServerConfig) (gomcp.Transport, error)

// Manager manages the lifecycle of MCP server connections.
type Manager struct {
	client           *gomcp.Client
	servers          []*serverEntry
	transportFactory TransportFactory
}

// NewManager creates a new Manager with the default transport factory.
func NewManager() *Manager {
	return &Manager{
		client: gomcp.NewClient(
			&gomcp.Implementation{Name: "rune-agent", Version: "v1.0.0"},
			nil,
		),
		transportFactory: defaultTransportFactory,
	}
}

// NewManagerWithTransport creates a new Manager using the given transport factory
// instead of the default one. This is useful for testing with in-memory transports.
func NewManagerWithTransport(factory TransportFactory) *Manager {
	return &Manager{
		client: gomcp.NewClient(
			&gomcp.Implementation{Name: "rune-agent", Version: "v1.0.0"},
			nil,
		),
		transportFactory: factory,
	}
}

// Connect connects to all configured MCP servers and returns their tools.
// Failed servers are logged and skipped.
func (m *Manager) Connect(ctx context.Context, cfg Config) []agent.Tool {
	var tools []agent.Tool
	for name, serverCfg := range cfg.MCPServers {
		serverTools := m.connectServer(ctx, name, serverCfg)
		tools = append(tools, serverTools...)
	}
	return tools
}

func (m *Manager) connectServer(
	ctx context.Context, name string, cfg ServerConfig,
) []agent.Tool {
	log := slog.With("server", name)

	info := &ServerInfo{
		Name:      name,
		Command:   cfg.Command,
		toolStats: make(map[string]*ToolStats),
	}
	entry := &serverEntry{name: name, config: cfg, info: info}
	m.servers = append(m.servers, entry)

	transport, err := m.transportFactory(name, cfg)
	if err != nil {
		log.Warn("mcp: unsupported server config", "error", err)
		info.Status = StatusError
		info.Error = err.Error()
		return nil
	}

	connCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	session, err := m.client.Connect(connCtx, transport, nil)
	if err != nil {
		log.Warn("mcp: failed to connect", "error", err)
		info.Status = StatusError
		info.Error = err.Error()
		return nil
	}
	entry.session = session

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		log.Warn("mcp: failed to list tools", "error", err)
		info.Status = StatusError
		info.Error = err.Error()
		return nil
	}

	info.Status = StatusConnected
	info.ConnectedAt = time.Now()
	info.ToolCount = len(result.Tools)

	toolNames := make([]string, 0, len(result.Tools))
	tools := make([]agent.Tool, 0, len(result.Tools))
	for _, t := range result.Tools {
		stats := &ToolStats{}
		info.toolStats[t.Name] = stats
		toolNames = append(toolNames, t.Name)
		tools = append(tools, &toolAdapter{
			serverName: name,
			toolName:   t.Name,
			mcpTool:    t,
			session:    session,
			stats:      stats,
		})
		log.Debug("mcp: registered tool", "tool", name+"_"+t.Name)
	}
	slices.Sort(toolNames)
	info.ToolNames = toolNames

	log.Info("mcp: connected", "tools", len(tools))
	return tools
}

// Servers returns a snapshot of all known servers, sorted by name.
func (m *Manager) Servers() []*ServerInfo {
	infos := make([]*ServerInfo, len(m.servers))
	for i, s := range m.servers {
		infos[i] = s.info
	}
	slices.SortFunc(infos, func(a, b *ServerInfo) int {
		return strings.Compare(a.Name, b.Name)
	})
	return infos
}

// Close closes all active sessions.
func (m *Manager) Close() error {
	var firstErr error
	for _, s := range m.servers {
		s.info.Status = StatusDisconnected
		if s.session == nil {
			continue
		}
		if err := s.session.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	m.servers = nil
	return firstErr
}

func defaultTransportFactory(_ string, cfg ServerConfig) (gomcp.Transport, error) {
	cmd := exec.Command(cfg.Command, cfg.Args...)
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	return &gomcp.CommandTransport{Command: cmd}, nil
}
