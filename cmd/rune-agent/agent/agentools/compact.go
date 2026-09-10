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

package agentools

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/rune/cmd/rune-agent/agent"
)

type compactTool struct{}

func newCompact() agent.Tool { return &compactTool{} }

func (t *compactTool) NeedsDeterministicOrder() bool { return false }

func (t *compactTool) Definition() llmapi.Tool {
	return llmapi.Tool{
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{
			Name: "compact",
			Description: "Compact the conversation by summarizing it into a fresh dialogue. " +
				"Use this when the conversation has grown large and older tool results " +
				"are no longer needed verbatim. The current conversation is preserved " +
				"immutably; a new one is created from the summary.",
			Parameters: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"required":             []string{},
				"additionalProperties": false,
			},
		},
	}
}

func (t *compactTool) Execute(_ context.Context, _ string) agent.ToolResult {
	return agent.ToolResult{Content: "Compacting conversation...", Compact: true}
}

func (t *compactTool) Summary(_ string) string { return "" }
