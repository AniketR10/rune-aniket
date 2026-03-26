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

package agent

import (
	"context"
	"slices"
	"strings"
	"time"

	"unstable.build/go-tui/cmd/rune-agent/llm"
)

// ToolResult is the result of executing a tool.
type ToolResult struct {
	Content           string
	MultiContent      []llm.ContentPart // Image/multi-modal content for user-message injection.
	IsError           bool
	Compact           bool     // Signals that the agent should compact the conversation.
	ClearContext      bool     // Signals that the agent should clear the conversation context.
	DropToolResultIDs []string // Tool call IDs whose results should be truncated in history.
	TouchedFiles      []string // Files that were created or modified (not deleted) by this tool.
}

// Tool is a capability that the agent can invoke via the LLM.
type Tool interface {
	Definition() llm.Tool
	Execute(ctx context.Context, arguments string) ToolResult
	// Summary returns a short human-readable description of the
	// arguments for display in the TUI. On parse error it returns
	// an empty string and the TUI falls back to formatToolArgs.
	Summary(arguments string) string
}

// Registry holds available tools and provides lookup.
// It supports provider-specific overrides: tools can be replaced or
// excluded for a given provider (e.g. "openai", "anthropic").
type Registry struct {
	tools     map[string]Tool
	overrides map[string]map[string]Tool // provider -> name -> Tool (nil = exclude)
}

// NewRegistry creates a Registry from the given tools.
func NewRegistry(tools ...Tool) *Registry {
	r := &Registry{
		tools:     make(map[string]Tool, len(tools)),
		overrides: make(map[string]map[string]Tool),
	}
	for _, t := range tools {
		r.tools[t.Definition().Function.Name] = t
	}
	return r
}

// RegisterOverrides adds provider-specific tool entries. If the tool
// name matches a base tool it replaces it for that provider; otherwise
// the tool is added only for that provider.
func (r *Registry) RegisterOverrides(provider string, tools ...Tool) {
	m := r.overrides[provider]
	if m == nil {
		m = make(map[string]Tool, len(tools))
		r.overrides[provider] = m
	}
	for _, t := range tools {
		m[t.Definition().Function.Name] = t
	}
}

// RegisterExclusions marks base tools as excluded for the given provider.
func (r *Registry) RegisterExclusions(provider string, names ...string) {
	m := r.overrides[provider]
	if m == nil {
		m = make(map[string]Tool, len(names))
		r.overrides[provider] = m
	}
	for _, name := range names {
		m[name] = nil
	}
}

// resolvedTools returns the merged tool map for the given provider.
// Empty provider returns base tools only.
func (r *Registry) resolvedTools(provider string) map[string]Tool {
	if provider == "" || len(r.overrides[provider]) == 0 {
		return r.tools
	}
	merged := make(map[string]Tool, len(r.tools))
	for k, v := range r.tools {
		merged[k] = v
	}
	for name, tool := range r.overrides[provider] {
		if tool == nil {
			delete(merged, name)
		} else {
			merged[name] = tool
		}
	}
	return merged
}

// CopyOverridesFrom copies all provider overrides (including exclusions)
// from src into r. Existing entries in r for the same provider+name are
// replaced.
func (r *Registry) CopyOverridesFrom(src *Registry) {
	for provider, srcTools := range src.overrides {
		m := r.overrides[provider]
		if m == nil {
			m = make(map[string]Tool, len(srcTools))
			r.overrides[provider] = m
		}
		for name, tool := range srcTools {
			m[name] = tool
		}
	}
}

// Get returns the tool with the given name, resolved for provider.
// Empty provider uses base tools only.
func (r *Registry) Get(name, provider string) (Tool, bool) {
	resolved := r.resolvedTools(provider)
	t, ok := resolved[name]
	return t, ok
}

// WithFilteredTools returns a new Registry containing only the tools whose
// names appear in allowedNames. Unknown names are silently ignored.
// Provider overrides are preserved and filtered to matching names.
func (r *Registry) WithFilteredTools(allowedNames []string) *Registry {
	allowed := make(map[string]bool, len(allowedNames))
	for _, name := range allowedNames {
		allowed[name] = true
	}
	filtered := &Registry{
		tools:     make(map[string]Tool, len(allowedNames)),
		overrides: make(map[string]map[string]Tool),
	}
	for _, name := range allowedNames {
		if t, ok := r.tools[name]; ok {
			filtered.tools[name] = t
		}
	}
	for provider, provTools := range r.overrides {
		for name, tool := range provTools {
			if !allowed[name] {
				continue
			}
			// Skip exclusions (nil) for explicitly allowed tools —
			// if a tool is in the allowed list, the caller wants it
			// available regardless of parent-level provider exclusions.
			if tool == nil {
				continue
			}
			if filtered.overrides[provider] == nil {
				filtered.overrides[provider] = make(map[string]Tool)
			}
			filtered.overrides[provider][name] = tool
		}
	}
	return filtered
}

// Tools returns llm.Tool definitions for all registered tools,
// resolved for provider, sorted by name for deterministic ordering.
// Stable ordering is critical for prompt caching — the cache
// breakpoint is on the last tool definition.
func (r *Registry) Tools(provider string) []llm.Tool {
	resolved := r.resolvedTools(provider)
	ret := make([]llm.Tool, 0, len(resolved))
	for _, t := range resolved {
		ret = append(ret, t.Definition())
	}
	sortTools(ret)
	return ret
}

// AllTools returns llm.Tool definitions for all base tools,
// ignoring provider overrides, sorted by name.
func (r *Registry) AllTools() []llm.Tool {
	ret := make([]llm.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		ret = append(ret, t.Definition())
	}
	sortTools(ret)
	return ret
}

func sortTools(tools []llm.Tool) {
	slices.SortFunc(tools, func(a, b llm.Tool) int {
		return strings.Compare(a.Function.Name, b.Function.Name)
	})
}

// EventType describes the kind of event emitted by the agent loop.
type EventType int

const (
	// EventText is a streamed text chunk from the assistant.
	EventText             EventType = iota // Streamed text chunk from assistant
	// EventToolCall reports that the agent is invoking a tool.
	EventToolCall                          // Agent is invoking a tool
	// EventToolResult reports that a tool finished executing.
	EventToolResult                        // Tool finished executing
	// EventDone reports that the agent finished.
	EventDone                              // Agent finished (final response complete)
	// EventError reports that an error occurred.
	EventError                             // Error occurred
	// EventReasoning is a streamed reasoning chunk from reasoning models.
	EventReasoning                         // Streamed reasoning chunk from reasoning models
	// EventCompacting indicates compaction is in progress.
	EventCompacting                        // Compaction in progress (animation hint)
	// EventCompacted reports that compaction finished and a new dialogue was created.
	EventCompacted                         // Compaction done, new dialogue created
	// EventRateLimitWarning reports a provider rate limit warning.
	EventRateLimitWarning                  // Rate limit warning from provider
	// EventUsageUpdate reports cumulative token usage after a completion.
	EventUsageUpdate                       // Cumulative token usage update after each completion
	// EventInferenceStart reports that an inference request is about to be sent.
	EventInferenceStart                    // Inference request about to be sent to LLM
	// EventInferenceReady reports that the inference stream is ready.
	EventInferenceReady                    // Inference stream created, waiting for first token
	// EventFirstContent reports receipt of the first content token.
	EventFirstContent                      // First content token received from LLM
	// EventToolsStart reports that tool execution is beginning.
	EventToolsStart                        // Tool execution phase beginning
	// EventToolsDropped reports that tool results were dropped from history.
	EventToolsDropped                      // Tool results were dropped from history
	// EventMemoryRecall reports that memory recall completed for the conversation.
	EventMemoryRecall                      // A memory was recalled for this conversation
)

// Event is emitted by the agent loop to drive the TUI.
type Event struct {
	Type               EventType
	Text               string             // For EventText: the text chunk
	Reasoning          string             // For EventReasoning: reasoning chunk
	ToolCallID         string             // For EventToolCall/EventToolResult: unique call identifier
	ToolName           string             // For EventToolCall/EventToolResult
	ToolArgs           string             // For EventToolCall: JSON arguments
	ToolSummary        string             // For EventToolCall/EventToolResult: short human-readable args summary
	ToolOutput         string             // For EventToolResult: execution output
	IsError            bool               // For EventToolResult: was it an error?
	ToolStartTime      time.Time          // For EventToolCall: when the tool call was announced
	ToolDuration       time.Duration      // For EventToolResult: how long the tool took
	Error              error              // For EventError
	RateLimit          *llm.RateLimitInfo // For EventRateLimitWarning
	Usage              llm.DialogueUsage  // For EventUsageUpdate: cumulative token usage
	DroppedToolCallIDs []string           // For EventToolsDropped: tool call IDs removed from history
	ArchivedDialogueID string             // For EventCompacted: ID under which old messages were archived
	Context            ContextSnapshot    // For EventUsageUpdate/EventDone: point-in-time context state
	Memories           []Memory          // For EventMemoryRecall: recalled memories
	MemoryDuration     time.Duration     // For EventMemoryRecall: how long recall took
}

// ContextSnapshot captures the point-in-time state of the conversation
// context at the moment an event is emitted.
type ContextSnapshot struct {
	TokensSent     int // Tokens sent to the model (conversation history + request)
	TokensReceived int // Tokens received from the model (completion output)
	Window         int // Model's context window size
	AutoCompactAt  int // Token count at which auto-compaction triggers
}
