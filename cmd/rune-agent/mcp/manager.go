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
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/cmd/rune-agent/agent"
)

// connectTimeout bounds the MCP initialize handshake only. It must not
// cover process startup, which can block on a user authorization
// prompt. Overridden in tests.
var connectTimeout = 30 * time.Second

// ServerStatus represents the current connection state of an MCP server.
type ServerStatus string

const (
	// StatusConnected indicates that the MCP server is connected.
	StatusConnected ServerStatus = "connected"
	// StatusConnecting indicates that the MCP server is starting up and
	// may be waiting on command authorization.
	StatusConnecting ServerStatus = "connecting"
	// StatusError indicates that the MCP server encountered an error.
	StatusError ServerStatus = "error"
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
//
// Servers are connected in the background while the agent is already
// running, so every access to the server list is guarded.
type Manager struct {
	client           *gomcp.Client
	mu               sync.Mutex
	servers          []*serverEntry
	transportFactory TransportFactory

	// OnServerExit, if set before connecting, observes connected
	// servers dying unexpectedly. Called outside the manager lock.
	OnServerExit ExitFunc
}

// NewManager creates a new Manager that starts stdio MCP servers through
// exec, rooted at dir. Routing through the workspace executor keeps every
// launch subject to the IDE's StartCommand authorization.
func NewManager(exec workspaceapi.Executor, dir string) *Manager {
	m := NewManagerWithTransport(nil)
	m.transportFactory = ExecutorTransport(exec, dir, m.serverExited)
	return m
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
		serverTools, _ := m.ConnectServer(ctx, name, serverCfg)
		tools = append(tools, serverTools...)
	}
	return tools
}

// ConnectServer connects to a single configured MCP server and returns
// its tools. The error reports why the server is unavailable; the
// failure is also recorded in the server's status.
func (m *Manager) ConnectServer(
	ctx context.Context, name string, cfg ServerConfig,
) ([]agent.Tool, error) {
	log := slog.With("server", name)

	info := &ServerInfo{
		Name:      name,
		Command:   cfg.Command,
		Status:    StatusConnecting,
		toolStats: make(map[string]*ToolStats),
	}
	entry := &serverEntry{name: name, config: cfg, info: info}
	m.mu.Lock()
	m.servers = append(m.servers, entry)
	m.mu.Unlock()

	transport, err := m.transportFactory(name, cfg)
	if err != nil {
		log.Warn("mcp: unsupported server config", "error", err)
		m.fail(info, err)
		return nil, err
	}

	connCtx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()

	session, err := m.client.Connect(connCtx, transport, nil)
	if err != nil {
		if detail := transportExitDetail(transport); detail != "" {
			err = fmt.Errorf("server %s", detail)
		}
		log.Warn("mcp: failed to connect", "error", err)
		closeTransport(transport)
		m.fail(info, err)
		return nil, err
	}
	m.mu.Lock()
	entry.session = session
	m.mu.Unlock()

	result, err := session.ListTools(ctx, nil)
	if err != nil {
		log.Warn("mcp: failed to list tools", "error", err)
		m.fail(info, err)
		return nil, err
	}

	toolNames := make([]string, 0, len(result.Tools))
	tools := make([]agent.Tool, 0, len(result.Tools))
	stats := make(map[string]*ToolStats, len(result.Tools))
	for _, t := range result.Tools {
		toolStats := &ToolStats{}
		stats[t.Name] = toolStats
		toolNames = append(toolNames, t.Name)
		tools = append(tools, &toolAdapter{
			serverName: name,
			toolName:   t.Name,
			mcpTool:    t,
			session:    session,
			stats:      toolStats,
		})
		log.Debug("mcp: registered tool", "tool", name+"_"+t.Name)
	}
	slices.Sort(toolNames)

	m.mu.Lock()
	info.Status = StatusConnected
	info.ConnectedAt = time.Now()
	info.ToolCount = len(result.Tools)
	info.ToolNames = toolNames
	info.toolStats = stats
	m.mu.Unlock()

	log.Info("mcp: connected", "tools", len(tools))
	return tools, nil
}

// serverExited transitions a connected server to the error state when
// its process dies without being asked to, and forwards the event.
func (m *Manager) serverExited(name, detail string) {
	m.mu.Lock()
	var unexpected bool
	for _, s := range m.servers {
		if s.name != name || s.info.Status != StatusConnected {
			continue
		}
		s.info.Status = StatusError
		s.info.Error = detail
		unexpected = true
	}
	onExit := m.OnServerExit
	m.mu.Unlock()

	if unexpected {
		slog.Warn("mcp: server exited", "server", name, "detail", detail)
		if onExit != nil {
			onExit(name, detail)
		}
	}
}

// fail records a terminal error for a server.
func (m *Manager) fail(info *ServerInfo, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	info.Status = StatusError
	info.Error = err.Error()
}

// closeTransport tears down a transport that owns an already-running
// server process but never reached a usable session.
func closeTransport(t gomcp.Transport) {
	if c, ok := t.(io.Closer); ok {
		_ = c.Close()
	}
}

// transportExitDetail asks a transport why its server process died.
// Empty when the transport does not track the process or it is alive.
func transportExitDetail(t gomcp.Transport) string {
	if d, ok := t.(interface{ ExitDetail() string }); ok {
		return d.ExitDetail()
	}
	return ""
}

// Servers returns a snapshot of all known servers, sorted by name. The
// returned values are copies: connection state may still be changing on
// the goroutine that is bringing the servers up.
func (m *Manager) Servers() []*ServerInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	infos := make([]*ServerInfo, len(m.servers))
	for i, s := range m.servers {
		snapshot := *s.info
		infos[i] = &snapshot
	}
	slices.SortFunc(infos, func(a, b *ServerInfo) int {
		return strings.Compare(a.Name, b.Name)
	})
	return infos
}

// Close closes all active sessions. Server entries are retained so that
// status remains inspectable after shutdown.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var firstErr error
	for _, s := range m.servers {
		s.info.Status = StatusDisconnected
		if s.session == nil {
			continue
		}
		if err := s.session.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		s.session = nil
	}
	return firstErr
}
