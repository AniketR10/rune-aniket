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
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/llm"
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

func (t *toolAdapter) Definition() llm.Tool {
	return llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name:        t.serverName + "_" + t.mcpTool.Name,
			Description: t.mcpTool.Description,
			Parameters:  sanitizeMCPInputSchema(t.serverName, t.mcpTool.Name, t.mcpTool.InputSchema),
		},
	}
}

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
