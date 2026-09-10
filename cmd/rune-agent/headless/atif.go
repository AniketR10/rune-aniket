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

package headless

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/rune/cmd/rune-agent/agent"
)

// SchemaVersion is the ATIF schema identifier emitted by headless runs.
// It must stay byte-identical to what atifact emits for the other
// harnesses, otherwise the trajectories are not comparable.
const SchemaVersion = "ATIF-v1.7"

// Step sources defined by ATIF.
const (
	SourceUser   = "user"
	SourceAgent  = "agent"
	SourceSystem = "system"
)

// Trajectory is the ATIF root envelope.
type Trajectory struct {
	SchemaVersion string         `json:"schema_version"`
	SessionID     string         `json:"session_id,omitempty"`
	Agent         Agent          `json:"agent"`
	Steps         []Step         `json:"steps"`
	Notes         string         `json:"notes,omitempty"`
	FinalMetrics  *FinalMetrics  `json:"final_metrics,omitempty"`
	Extra         map[string]any `json:"extra,omitempty"`
}

// Agent describes the harness that produced the trajectory.
type Agent struct {
	Name            string           `json:"name"`
	Version         string           `json:"version"`
	ModelName       string           `json:"model_name,omitempty"`
	ToolDefinitions []ToolDefinition `json:"tool_definitions,omitempty"`
	Extra           map[string]any   `json:"extra,omitempty"`
}

// ToolDefinition is an OpenAI-shaped tool declaration.
type ToolDefinition struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction carries the callable half of a ToolDefinition.
type ToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

// Step is one turn of the trajectory.
type Step struct {
	StepID           int            `json:"step_id"`
	Timestamp        string         `json:"timestamp,omitempty"`
	Source           string         `json:"source"`
	ModelName        string         `json:"model_name,omitempty"`
	ReasoningEffort  string         `json:"reasoning_effort,omitempty"`
	Message          string         `json:"message"`
	ReasoningContent string         `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall     `json:"tool_calls,omitempty"`
	Observation      *Observation   `json:"observation,omitempty"`
	Metrics          *Metrics       `json:"metrics,omitempty"`
	LLMCallCount     int            `json:"llm_call_count,omitempty"`
	Extra            map[string]any `json:"extra,omitempty"`
}

// ToolCall records one tool invocation issued by the model.
type ToolCall struct {
	ToolCallID   string         `json:"tool_call_id"`
	FunctionName string         `json:"function_name"`
	Arguments    map[string]any `json:"arguments"`
}

// Observation groups the results of a step's tool calls.
type Observation struct {
	Results []ObservationResult `json:"results"`
}

// ObservationResult is one tool result, keyed back to its call.
type ObservationResult struct {
	SourceCallID string         `json:"source_call_id,omitempty"`
	Content      string         `json:"content,omitempty"`
	Extra        map[string]any `json:"extra,omitempty"`
}

// Metrics holds per-step token accounting.
type Metrics struct {
	PromptTokens     int            `json:"prompt_tokens"`
	CompletionTokens int            `json:"completion_tokens"`
	CachedTokens     int            `json:"cached_tokens"`
	CostUSD          *float64       `json:"cost_usd,omitempty"`
	Extra            map[string]any `json:"extra,omitempty"`
}

// FinalMetrics holds whole-run token accounting.
type FinalMetrics struct {
	TotalPromptTokens     int            `json:"total_prompt_tokens"`
	TotalCompletionTokens int            `json:"total_completion_tokens"`
	TotalCachedTokens     int            `json:"total_cached_tokens"`
	TotalCostUSD          *float64       `json:"total_cost_usd,omitempty"`
	TotalSteps            int            `json:"total_steps"`
	Extra                 map[string]any `json:"extra,omitempty"`
}

// BuilderConfig carries the run-level facts that do not change between
// steps.
type BuilderConfig struct {
	AgentVersion    string
	SessionID       string
	ModelName       string
	ReasoningEffort string
	ToolDefinitions []llmapi.Tool
	Extra           map[string]any
}

// Builder turns the agent event stream into an ATIF trajectory.
//
// The zero value is not usable; construct one with NewBuilder.
type Builder struct {
	cfg      BuilderConfig
	steps    []Step
	open     *openStep
	callStep map[string]int
	prev     llmapi.DialogueUsage
	status   string
	now      func() time.Time
}

// openStep accumulates the agent turn currently being assembled.
type openStep struct {
	timestamp   time.Time
	message     strings.Builder
	reasoning   strings.Builder
	toolCalls   []ToolCall
	observation []ObservationResult
}

// NewBuilder returns a Builder that emits a trajectory for a single
// headless run.
func NewBuilder(cfg BuilderConfig) *Builder {
	return &Builder{
		cfg:      cfg,
		callStep: make(map[string]int),
		now:      time.Now,
	}
}

// AddUserStep records the instructions as the opening user step. ATIF
// forbids agent-only fields on user steps, so nothing but the message is
// attached.
func (b *Builder) AddUserStep(message string) {
	b.steps = append(b.steps, Step{
		Source:    SourceUser,
		Timestamp: b.timestamp(),
		Message:   message,
	})
}

// AddEvent folds one agent event into the trajectory.
func (b *Builder) AddEvent(ev agent.Event) {
	switch ev.Type {
	case agent.EventText:
		b.ensureOpen().message.WriteString(ev.Text)
	case agent.EventReasoning:
		b.ensureOpen().reasoning.WriteString(ev.Reasoning)
	case agent.EventToolCall:
		open := b.ensureOpen()
		open.toolCalls = append(open.toolCalls, ToolCall{
			ToolCallID:   ev.ToolCallID,
			FunctionName: ev.ToolName,
			Arguments:    decodeArguments(ev.ToolArgs),
		})
		b.callStep[ev.ToolCallID] = len(b.steps)
	case agent.EventToolResult:
		b.addObservation(ev)
	case agent.EventUsageUpdate:
		b.closeStep(b.stepMetrics(ev.Usage))
	case agent.EventRefusal:
		b.closeStep(nil)
		b.addSystemStep("refused", "the model refused to continue")
	case agent.EventError:
		b.closeStep(nil)
		msg := "the agent loop terminated with an error"
		if ev.Error != nil {
			msg = ev.Error.Error()
		}
		b.addSystemStep("error", msg)
	}
}

// Finish closes any open step and returns the complete trajectory.
func (b *Builder) Finish() Trajectory {
	b.closeStep(nil)

	steps := b.steps
	if steps == nil {
		steps = []Step{}
	}
	for i := range steps {
		steps[i].StepID = i + 1
	}

	extra := b.cfg.Extra
	if b.status != "" {
		if extra == nil {
			extra = make(map[string]any, 1)
		}
		extra["status"] = b.status
	}

	return Trajectory{
		SchemaVersion: SchemaVersion,
		SessionID:     b.cfg.SessionID,
		Agent: Agent{
			Name:            "rune-agent",
			Version:         b.cfg.AgentVersion,
			ModelName:       b.cfg.ModelName,
			ToolDefinitions: toolDefinitions(b.cfg.ToolDefinitions),
		},
		Steps:        steps,
		FinalMetrics: finalMetrics(steps),
		Extra:        extra,
	}
}

func (b *Builder) ensureOpen() *openStep {
	if b.open == nil {
		b.open = &openStep{timestamp: b.now()}
	}
	return b.open
}

// addObservation attaches a tool result to whichever step issued the
// call. Results arrive after EventUsageUpdate has already closed the
// turn, so the target step is usually one that is already in b.steps.
func (b *Builder) addObservation(ev agent.Event) {
	res := ObservationResult{
		SourceCallID: ev.ToolCallID,
		Content:      ev.ToolOutput,
	}
	if ev.IsError {
		res.Extra = map[string]any{"is_error": true}
	}
	idx, ok := b.callStep[ev.ToolCallID]
	if !ok || idx == len(b.steps) {
		open := b.ensureOpen()
		open.observation = append(open.observation, res)
		return
	}
	step := &b.steps[idx]
	if step.Observation == nil {
		step.Observation = &Observation{}
	}
	step.Observation.Results = append(step.Observation.Results, res)
}

// closeStep materializes the open agent turn. metrics may be nil when
// the turn is closed by termination rather than by a completion.
func (b *Builder) closeStep(metrics *Metrics) {
	open := b.open
	if open == nil {
		return
	}
	b.open = nil

	step := Step{
		Timestamp:        open.timestamp.UTC().Format(time.RFC3339Nano),
		Source:           SourceAgent,
		ModelName:        b.cfg.ModelName,
		ReasoningEffort:  b.cfg.ReasoningEffort,
		Message:          open.message.String(),
		ReasoningContent: open.reasoning.String(),
		ToolCalls:        open.toolCalls,
		Metrics:          metrics,
	}
	if len(open.observation) > 0 {
		step.Observation = &Observation{Results: open.observation}
	}
	if metrics != nil {
		step.LLMCallCount = 1
	}
	b.steps = append(b.steps, step)
}

// addSystemStep records a terminal condition. ATIF forbids agent-only
// fields on system steps, so only the message is carried.
func (b *Builder) addSystemStep(status, message string) {
	b.status = status
	b.steps = append(b.steps, Step{
		Source:    SourceSystem,
		Timestamp: b.timestamp(),
		Message:   message,
	})
}

// stepMetrics differences the agent's cumulative dialogue usage into the
// per-step deltas ATIF expects.
func (b *Builder) stepMetrics(cur llmapi.DialogueUsage) *Metrics {
	// TokensSent already includes cache reads and cache creation for
	// every provider, which is exactly how atifact computes
	// prompt_tokens for the other harnesses.
	m := &Metrics{
		PromptTokens:     cur.TokensSent - b.prev.TokensSent,
		CompletionTokens: cur.TokensReceived - b.prev.TokensReceived,
		CachedTokens:     cur.TokensCached - b.prev.TokensCached,
	}
	if created := cur.TokensCacheCreated - b.prev.TokensCacheCreated; created > 0 {
		m.Extra = map[string]any{"cache_creation_input_tokens": created}
	}
	b.prev = cur
	return m
}

func (b *Builder) timestamp() string {
	return b.now().UTC().Format(time.RFC3339Nano)
}

// decodeArguments coerces the agent's JSON argument string into the
// object ATIF requires, preserving anything unparseable verbatim.
func decodeArguments(raw string) map[string]any {
	if raw == "" {
		return map[string]any{}
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(raw), &args); err != nil || args == nil {
		return map[string]any{"_raw": raw}
	}
	return args
}

func toolDefinitions(tools []llmapi.Tool) []ToolDefinition {
	if len(tools) == 0 {
		return nil
	}
	ret := make([]ToolDefinition, 0, len(tools))
	for _, t := range tools {
		ret = append(ret, ToolDefinition{
			Type: string(t.Type),
			Function: ToolFunction{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			},
		})
	}
	return ret
}

func finalMetrics(steps []Step) *FinalMetrics {
	ret := &FinalMetrics{TotalSteps: len(steps)}
	for _, s := range steps {
		if s.Metrics == nil {
			continue
		}
		ret.TotalPromptTokens += s.Metrics.PromptTokens
		ret.TotalCompletionTokens += s.Metrics.CompletionTokens
		ret.TotalCachedTokens += s.Metrics.CachedTokens
	}
	return ret
}
