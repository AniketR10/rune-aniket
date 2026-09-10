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
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/rune/cmd/rune-agent/agent"
)

// ToolStats tracks execution metrics for a single MCP tool.
// All fields are safe for concurrent use.
type ToolStats struct {
	callCount     atomic.Int64
	errorCount    atomic.Int64
	totalDuration atomic.Int64 // nanoseconds
	lastCall      atomic.Int64 // UnixNano
}

// CallCount returns the total number of calls.
func (s *ToolStats) CallCount() int64 { return s.callCount.Load() }

// ErrorCount returns the total number of failed calls.
func (s *ToolStats) ErrorCount() int64 { return s.errorCount.Load() }

// TotalDuration returns the cumulative call duration.
func (s *ToolStats) TotalDuration() time.Duration {
	return time.Duration(s.totalDuration.Load())
}

// LastCall returns the time of the most recent call, or zero.
func (s *ToolStats) LastCall() time.Time {
	ns := s.lastCall.Load()
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}

func (s *ToolStats) record(dur time.Duration, isError bool) {
	s.callCount.Add(1)
	s.totalDuration.Add(int64(dur))
	s.lastCall.Store(time.Now().UnixNano())
	if isError {
		s.errorCount.Add(1)
	}
}

// toolAdapter wraps an MCP tool as an agent.Tool.
type toolAdapter struct {
	serverName string
	toolName   string // original MCP tool name (used in CallTool)
	mcpTool    *gomcp.Tool
	session    *gomcp.ClientSession
	stats      *ToolStats
}

func (t *toolAdapter) Definition() llmapi.Tool {
	return llmapi.Tool{
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{
			Name:        t.serverName + "_" + t.mcpTool.Name,
			Description: t.mcpTool.Description,
			Parameters:  sanitizeMCPInputSchema(t.serverName, t.mcpTool.Name, t.mcpTool.InputSchema),
		},
	}
}

// NeedsDeterministicOrder conservatively serializes every MCP tool
// since the agent cannot know a dynamic server tool's side effects.
func (t *toolAdapter) NeedsDeterministicOrder() bool { return true }

func (t *toolAdapter) Execute(ctx context.Context, arguments string) agent.ToolResult {
	start := time.Now()

	var args map[string]any
	if arguments != "" {
		if err := json.Unmarshal([]byte(arguments), &args); err != nil {
			result := agent.ToolResult{
				Content: fmt.Sprintf("error: invalid arguments: %v", err),
				IsError: true,
			}
			if t.stats != nil {
				t.stats.record(time.Since(start), true)
			}
			return result
		}
	}

	mcpResult, err := t.session.CallTool(ctx, &gomcp.CallToolParams{
		Name:      t.toolName,
		Arguments: args,
	})
	dur := time.Since(start)

	if err != nil {
		result := agent.ToolResult{
			Content: fmt.Sprintf("error: %v", err),
			IsError: true,
		}
		if t.stats != nil {
			t.stats.record(dur, true)
		}
		return result
	}

	text := flattenContent(mcpResult.Content)
	result := agent.ToolResult{
		Content: text,
		IsError: mcpResult.IsError,
	}
	if t.stats != nil {
		t.stats.record(dur, mcpResult.IsError)
	}
	return result
}

func (t *toolAdapter) Summary(_ string) string {
	return ""
}

// flattenContent concatenates the text parts of MCP Content items.
func flattenContent(contents []gomcp.Content) string {
	var sb strings.Builder
	for i, c := range contents {
		tc, ok := c.(*gomcp.TextContent)
		if !ok {
			continue
		}
		if i > 0 && sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(tc.Text)
	}
	return sb.String()
}
