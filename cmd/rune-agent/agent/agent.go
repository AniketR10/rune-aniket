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
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
	"unstable.build/go-tui/cmd/rune-agent/llm"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// Config holds agent configuration.
type Config struct {
	MaxIterations      int
	MaxToolOutputBytes int     // 0 uses DefaultMaxToolOutputBytes.
	AutoCompactRatio   float64 // 0 uses defaultAutoCompactRatio.
	SystemPrompt       string
	SessionKey         string
	AgentID            string
	Model              string
	Provider           string // e.g. "openai", "anthropic", "gemini", "ollama"
	Workspace          workspaceapi.URI
	SubAgent           bool // true for sub-agent dialogues spawned by agent tool calls.

	// CompactSvc, when non-nil, is used for summarization during
	// compaction instead of the agent's own LLM service. This allows
	// using a cheaper/faster model for conversation summaries.
	CompactSvc llm.Service

	// ProjectInstructions holds the content loaded from project
	// instruction files (e.g. AGENTS.md). When non-empty it is
	// injected as a <project-instructions> XML block prepended to
	// the user message on every turn (not persisted).
	ProjectInstructions string
}

// Memory represents a single recalled memory entry.
type Memory struct {
	ID      string // e.g. "recent-iterator-bug"
	Content string // the memory knowledge text
}

// MemoryRecaller retrieves relevant memories for the current context.
// Implementations should return (nil, nil) when no memories match.
type MemoryRecaller interface {
	Recall(ctx context.Context, files []string, task string) ([]Memory, error)
}

// NoMemory returns a MemoryRecaller that always reports no memories.
// Use it for agents that have no memory system configured.
func NoMemory() MemoryRecaller { return noMemory{} }

type noMemory struct{}

func (noMemory) Recall(_ context.Context, _ []string, _ string) ([]Memory, error) {
	return nil, nil
}

// Agent orchestrates the agentic loop: LLM → tool call → result → repeat.
type Agent struct {
	mu            sync.Mutex // protects svc, config.Model, and effort
	svc           llm.Service
	registry      *Registry
	skillRegistry *skills.SkillRegistry
	store         dialoguemanager.Store
	resources     sync.Map
	config        Config
	effort        llm.ReasoningEffort // session-level effort override
	memory        MemoryRecaller
}

// SwapService replaces the LLM service, model, and provider used by the agent.
// It must be called between Run invocations, not during one.
func (a *Agent) SwapService(svc llm.Service, model, provider string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.svc = svc
	a.config.Model = model
	a.config.Provider = provider
}

// provider returns the current provider name under the mutex.
func (a *Agent) provider() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.config.Provider
}

// Model returns the current model name.
func (a *Agent) Model() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.config.Model
}

// SetEffort sets the session-level reasoning effort. An empty string
// means use the provider/config default.
func (a *Agent) SetEffort(effort llm.ReasoningEffort) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.effort = effort
}

// Effort returns the current session-level reasoning effort.
func (a *Agent) Effort() llm.ReasoningEffort {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.effort
}

// NewAgent creates a new Agent. Panics if skillRegistry is nil.
func NewAgent(
	svc llm.Service,
	registry *Registry,
	skillRegistry *skills.SkillRegistry,
	store dialoguemanager.Store,
	memory MemoryRecaller,
	config Config,
) *Agent {
	if skillRegistry == nil {
		panic("agent: skill registry must not be nil")
	}
	if memory == nil {
		panic("agent: memory must not be nil")
	}
	if config.MaxIterations <= 0 {
		config.MaxIterations = 500
	}
	return &Agent{
		svc:           svc,
		registry:      registry,
		skillRegistry: skillRegistry,
		store:         store,
		memory:        memory,
		config:        config,
	}
}

// AddContextResource adds an editor resource to the agent's context.
func (a *Agent) AddContextResource(
	ctx context.Context, uri workspaceapi.URI, data string,
) error {
	a.resources.Store(uri, data)
	return nil
}

// RemoveContextResource removes an editor resource from the agent's context.
func (a *Agent) RemoveContextResource(
	ctx context.Context, uri workspaceapi.URI,
) error {
	_, loaded := a.resources.LoadAndDelete(uri)
	if !loaded {
		return fmt.Errorf("resource with URI %q not found", uri.String())
	}
	return nil
}

// resourceFiles extracts file paths from the agent's context resources.
func (a *Agent) resourceFiles() []string {
	var files []string
	a.resources.Range(func(k, _ any) bool {
		if uri, ok := k.(workspaceapi.URI); ok {
			files = append(files, uri.Path())
		}
		return true
	})
	return files
}

// RunOption configures optional parameters for Agent.Run.
type RunOption func(*runOptions)

type runOptions struct {
	skillName       string
	toolCallResults []ToolCallResult
}

// ToolCallResult represents a pre-computed tool call result that is
// injected into the message history before the first LLM call. This
// avoids redundant tool invocations when the caller already knows
// what files the agent will read.
type ToolCallResult struct {
	ToolName  string
	Arguments string
	Content   string
}

type toolCallInfo struct {
	call    llm.ToolCall
	tool    Tool
	found   bool
	summary string
}

type executedToolCall struct {
	index    int
	info     toolCallInfo
	result   ToolResult
	duration time.Duration
}

// WithSkillName sets the skill pre-loaded via slash command.
func WithSkillName(name string) RunOption {
	return func(o *runOptions) {
		o.skillName = name
	}
}

// WithToolCallResults injects pre-computed tool call results into the
// message history. The results appear as if the agent had already
// called these tools before its first LLM turn.
func WithToolCallResults(results []ToolCallResult) RunOption {
	return func(o *runOptions) {
		o.toolCallResults = results
	}
}

// Run executes the agentic loop for the given dialogue and user message.
// It returns an iterator of Events that the caller consumes to drive the UI.
func (a *Agent) Run(
	ctx context.Context, dialogueID string, message string, opts ...RunOption,
) iterator.Iterator[Event] {
	var options runOptions
	for _, o := range opts {
		o(&options)
	}

	ch := make(chan Event, 16)
	it := &channelIterator{ch: ch}

	go func() {
		defer close(ch)
		a.run(ctx, ch, dialogueID, message, options)
	}()

	return it
}

func (a *Agent) run(
	ctx context.Context, ch chan<- Event,
	dialogueID string, userMessage string, opts runOptions,
) {
	ctx = WithActivatedSkills(ctx)

	// If a skill was pre-loaded via slash command, mark it activated
	// (prevents the LLM from re-invoking it via the use_skill tool)
	// and prepare its content for system-message injection.
	var preloadedSkillMsg string
	if opts.skillName != "" {
		if skill, ok := a.skillRegistry.Get(opts.skillName); ok {
			MarkSkillActivated(ctx, skill.Name)
			preloadedSkillMsg = skills.FormatSkillContent(skill)
		}
	}

	ctx = llm.WithAuditDialogueID(ctx, dialogueID)
	log := slog.With("struct", "agent.Agent", "dialogueID", dialogueID)

	maxOutput := a.config.MaxToolOutputBytes
	if maxOutput <= 0 {
		maxOutput = DefaultMaxToolOutputBytes
	}

	// Load or create dialogue
	var isNew bool
	dialogue, err := a.store.Get(ctx, dialogueID)
	if err != nil {
		if !errors.Is(err, storageapi.ErrNotFound) {
			emit(ctx, ch, Event{Type: EventError, Error: fmt.Errorf("get dialogue: %w", err)})
			return
		}
		isNew = true
		// New dialogue: seed with system prompt
		dialogue.ID = dialogueID
		dialogue.WorkspaceURI = a.config.Workspace.String()
		dialogue.Messages = []llm.Message{
			{Role: llm.RoleSystem, Content: a.config.SystemPrompt},
		}
		log.Debug("new dialogue: added system prompt", "prompt", a.config.SystemPrompt)
	} else {
		log.Debug("found dialogue in storage: re-using",
			"messages", len(dialogue.Messages), "version", dialogue.Version,
			"ID", dialogue.ID, "updated", dialogue.UpdatedAt)
	}

	// Gather context resources in deterministic order. sync.Map.Range
	// iterates non-deterministically; sorting by URI string ensures the
	// system prompt prefix is stable across calls, which is critical for
	// prompt caching.
	type resourceEntry struct {
		uri     string
		content string
	}
	var resources []resourceEntry
	a.resources.Range(func(k, v any) bool {
		resources = append(resources, resourceEntry{
			uri:     fmt.Sprint(k),
			content: v.(string),
		})
		return true
	})
	sort.Slice(resources, func(i, j int) bool {
		return resources[i].uri < resources[j].uri
	})
	resourceMsgs := make([]llm.Message, 0, len(resources))
	for _, r := range resources {
		resourceMsgs = append(resourceMsgs, llm.Message{
			Role: llm.RoleSystem,
			Content: fmt.Sprintf("The file with URI %s is "+
				"in the user's context:\n```\n%s\n```", r.uri, r.content),
		})
		log.Debug("added file to context", "uri", r.uri, "size", len(r.content))
	}

	// Append user message
	userMsg := llm.Message{Role: llm.RoleUser, Content: userMessage}

	// Build full message list
	messages := make([]llm.Message, 0,
		len(dialogue.Messages)+len(resourceMsgs)+1)
	messages = append(messages, dialogue.Messages...)
	messages = normalizeMessages(messages)
	messages = append(messages, resourceMsgs...)
	messages = append(messages, userMsg)

	// Track new messages for persistence.
	// For new dialogues, include the seeded system prompt so it is persisted.
	newMessages := make([]llm.Message, 0, len(dialogue.Messages)+1)
	if isNew {
		newMessages = append(newMessages, dialogue.Messages...)
	}
	newMessages = append(newMessages, userMsg)

	// Inject pre-computed tool call results so the agent starts with
	// these files already "read" — avoids redundant tool invocations.
	if len(opts.toolCallResults) > 0 {
		tcMsgs := buildToolCallMessages(opts.toolCallResults)
		messages = append(messages, tcMsgs...)
		newMessages = append(newMessages, tcMsgs...)
	}

	// defaultAutoCompactRatio is the fraction of the context window above
	// which the runtime auto-compacts without waiting for the model.
	const defaultAutoCompactRatio = 0.85

	autoCompactRatio := a.config.AutoCompactRatio
	if autoCompactRatio <= 0 {
		autoCompactRatio = defaultAutoCompactRatio
	}

	// Track usage across the entire Run invocation.
	var usage llm.DialogueUsage
	runStart := time.Now()

	contextWindow := a.svc.ContextWindow()

	// lastAPITokensSent caches the sent token count reported by the
	// provider in its most recent response. When available it is more
	// accurate than the local tiktoken estimate (especially for
	// non-OpenAI providers), so we prefer it for context-usage checks.
	// Falls back to CountTokens on the first iteration.
	var lastAPITokensSent int

	// response and reasoningResponse accumulate streamed text for the
	// fallback assistant message when doneData is absent. Declared outside
	// the loop so their internal buffers are reused across turns rather
	// than re-grown from zero on every iteration.
	var response strings.Builder
	var reasoningResponse strings.Builder

	// Recall relevant memories for this user message. The content is
	// injected into the user message at a fixed index inside the loop
	// (see below). Memory is recalled once per Run; "every turn" means
	// every user message / Run() call.
	recallStart := time.Now()
	memories, memErr := a.memory.Recall(ctx, a.resourceFiles(), userMessage)
	recallDuration := time.Since(recallStart)
	if memErr != nil {
		log.Warn("memory recall failed", "error", memErr)
	} else {
		log.Debug("memory recall", "count", len(memories), "duration", recallDuration)
	}
	if len(memories) > 0 {
		emit(ctx, ch, Event{Type: EventMemoryRecall, Memories: memories, MemoryDuration: recallDuration})
	}
	memoryContent := FormatMemoryContent(memories)

	// userMsgIdx is the position of the user's message in `messages`.
	// Memory and project instructions are injected at this index every
	// iteration so the prefix content is stable for prompt caching.
	// The index accounts for any pre-computed tool results appended
	// after the user message, and is updated after compaction resets
	// `messages` to a shorter slice.
	userMsgIdx := len(messages) - 1
	if len(opts.toolCallResults) > 0 {
		userMsgIdx -= len(buildToolCallMessages(opts.toolCallResults))
	}

	// Snapshot tools once before the loop. Registry.Tools iterates
	// over a map, so calling it per-iteration would produce
	// non-deterministic ordering that busts the tools cache.
	tools := a.registry.Tools(a.provider())

	var transient []llm.Message
	var infos []toolCallInfo
	var toolMsgs []llm.Message
	var imageContentParts []llm.ContentPart
	for i := range a.config.MaxIterations {
		log.Debug("agent loop iteration",
			"iteration", i, "messages", len(messages))

		// Build request messages with transient injections.
		// These are not persisted — they reflect live state each turn.
		reqMessages := messages

		// Re-scan skill directories so out-of-band installations
		// (e.g. after request_skill) are picked up immediately.
		// The skills list is sorted by name, so content is stable
		// unless a skill is actually installed or removed.
		a.skillRegistry.Reload()

		transient = transient[:0]
		if section := skillsPromptSection(a.skillRegistry.List()); section != "" {
			transient = append(transient, llm.Message{Role: llm.RoleSystem, Content: section})
		}
		if preloadedSkillMsg != "" {
			transient = append(transient, llm.Message{Role: llm.RoleSystem, Content: preloadedSkillMsg})
		}
		if len(transient) > 0 {
			req := make([]llm.Message, 0, len(reqMessages)+len(transient))
			req = append(req, reqMessages[0])
			req = append(req, transient...)
			req = append(req, reqMessages[1:]...)
			reqMessages = req
		}

		// Inject project instructions and memory context into the user
		// message. Both are transient — NOT persisted. userMsgIdx
		// tracks the user message position and is updated after
		// compaction; transient insertion shifts it uniformly.
		if a.config.ProjectInstructions != "" || memoryContent != "" {
			if len(reqMessages) == len(messages) {
				reqMessages = slices.Clone(reqMessages)
			}
			idx := userMsgIdx
			if len(transient) > 0 {
				idx += len(transient)
			}
			content := reqMessages[idx].Content
			if memoryContent != "" {
				content = "<memory-context>\n" + memoryContent +
					"\n</memory-context>\n\n" + content
			}
			if a.config.ProjectInstructions != "" {
				content = "<project-instructions>\n" + a.config.ProjectInstructions +
					"\n</project-instructions>\n\n" + content
			}
			reqMessages[idx] = llm.Message{
				Role:    llm.RoleUser,
				Content: content,
			}
		}

		// Inject transient hint when approaching the context window limit.
		// Prefer the provider-reported sent token count from the last
		// completion — it reflects the actual tokenizer. Fall back to
		// the local tiktoken estimate on the first iteration or when
		// the provider did not report usage.
		tokenCount := lastAPITokensSent
		if tokenCount == 0 {
			tokenCount, _ = a.svc.CountTokens(reqMessages)
			tokenCount += estimateToolDefTokens(tools)
		}
		contextUsage := float64(tokenCount) / float64(contextWindow)
		log.Debug("context window usage",
			"tokens", tokenCount,
			"context_window", contextWindow,
			"usage_pct", int(contextUsage*100),
		)

		// Auto-compact: if usage exceeds the auto-compact ratio, compact
		// without waiting for the model to call the compact tool.
		if contextWindow > 0 && contextUsage >= autoCompactRatio {
			log.Info("auto-compacting: context usage above threshold",
				"usage_pct", int(contextUsage*100),
				"threshold_pct", int(autoCompactRatio*100),
			)
			emit(ctx, ch, Event{Type: EventCompacting})
			dialogue.Messages = messages
			compactedMsgs, compactErr := a.compact(ctx, ch, dialogue)
			if compactErr != nil {
				log.Warn("auto-compact failed, continuing without compaction", "error", compactErr)
			} else {
				dialogue = dialoguemanager.Dialogue{ID: dialogueID, Messages: compactedMsgs, WorkspaceURI: dialogue.WorkspaceURI, Version: 1}
				newMessages = nil
				messages = append(compactedMsgs[:len(compactedMsgs):len(compactedMsgs)], resourceMsgs...)
				userMsgIdx = len(compactedMsgs) - 1
				lastAPITokensSent = 0
				emit(ctx, ch, Event{Type: EventDone})
				continue
			}
		}

		req := llm.Request{
			Messages:        reqMessages,
			PromptCacheKey:  dialogueID,
			Tools:           tools,
			ReasoningEffort: a.Effort(),
			TokenCount:      lastAPITokensSent,
		}

		emit(ctx, ch, Event{Type: EventInferenceStart})
		inferenceStart := time.Now()
		log.Debug("creating completion", "messages", len(reqMessages), "tools", len(tools))
		it, err := a.svc.CreateCompletion(ctx, req)
		log.Debug("created completion", "error", err, "duration", time.Since(inferenceStart))
		if err != nil {
			emit(ctx, ch, Event{Type: EventError, Error: fmt.Errorf("create completion: %w", err)})
			usage.TotalDuration = time.Since(runStart)
			a.persistMessages(ctx, dialogueID, dialogue, newMessages, usage)
			return
		}
		emit(ctx, ch, Event{Type: EventInferenceReady})

		response.Reset()
		reasoningResponse.Reset()
		var doneData *llm.DoneData
		firstContent := false

		// Consume stream
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			switch ev.Type {
			case llm.EventTextDelta:
				if !firstContent {
					firstContent = true
					emit(ctx, ch, Event{Type: EventFirstContent})
				}
				emit(ctx, ch, Event{Type: EventText, Text: ev.Text})
				response.WriteString(ev.Text)
			case llm.EventReasoningDelta:
				if !firstContent {
					firstContent = true
					emit(ctx, ch, Event{Type: EventFirstContent})
				}
				emit(ctx, ch, Event{Type: EventReasoning, Reasoning: ev.Reasoning})
				reasoningResponse.WriteString(ev.Reasoning)
			case llm.EventToolCallDone:
				// Streaming hint — full tool calls come via doneData.
			case llm.EventStreamDone:
				doneData = ev.DoneData
			case llm.EventRateLimitWarning:
				emit(ctx, ch, Event{Type: EventRateLimitWarning, RateLimit: ev.RateLimit})
			case llm.EventStreamReset:
				response.Reset()
				reasoningResponse.Reset()
				doneData = nil
				firstContent = false
				emit(ctx, ch, Event{Type: EventDone})
				emit(ctx, ch, Event{Type: EventInferenceStart})
			case llm.EventStreamError:
				log.Debug("stream error during inference",
					"error", ev.Error,
					"messages_sent", len(reqMessages),
					"response_so_far_len", response.Len(),
					"reasoning_so_far_len", reasoningResponse.Len(),
					"elapsed", time.Since(inferenceStart),
				)
				emit(ctx, ch, Event{Type: EventError, Error: fmt.Errorf("stream: %w", ev.Error)})
				usage.TotalDuration = time.Since(runStart)
				a.persistMessages(ctx, dialogueID, dialogue, newMessages, usage)
				_ = it.Close()
				return
			}
		}
		inferenceDuration := time.Since(inferenceStart)

		err = it.Err()
		log.Debug("consumed completion response", "error", err, "duration", inferenceDuration)
		if err != nil {
			emit(ctx, ch, Event{Type: EventError, Error: fmt.Errorf("stream: %w", err)})
			usage.TotalDuration = time.Since(runStart)
			a.persistMessages(ctx, dialogueID, dialogue, newMessages, usage)
			return
		}
		_ = it.Close()

		// Build assistant message from doneData (source of truth).
		var assistantMsg llm.Message
		var finishReason llm.FinishReason
		var completionUsage llm.Usage
		if doneData != nil {
			assistantMsg = doneData.Message
			finishReason = doneData.FinishReason
			completionUsage = doneData.Usage
		} else {
			assistantMsg = llm.Message{
				Role:             llm.RoleAssistant,
				Content:          response.String(),
				ReasoningContent: reasoningResponse.String(),
			}
		}
		// Cache provider-reported sent tokens for the next iteration's
		// context-usage check. Reset to 0 after compaction so the next
		// iteration falls back to CountTokens with the new message set.
		lastAPITokensSent = completionUsage.TokensSent

		messages = append(messages, assistantMsg)
		newMessages = append(newMessages, assistantMsg)

		// Emit a snapshot of cumulative usage after each completion so
		// the TUI can display token counts in the status hint.
		{
			snapshot := usage
			snapshot.Add(completionUsage, 0, inferenceDuration, 0)
			snapshot.TotalDuration = time.Since(runStart)
			emit(ctx, ch, Event{
				Type:  EventUsageUpdate,
				Usage: snapshot,
				Context: ContextSnapshot{
					TokensSent:     completionUsage.TokensSent,
					TokensReceived: completionUsage.TokensReceived,
					Window:         contextWindow,
					AutoCompactAt:  int(float64(contextWindow) * autoCompactRatio),
				},
			})
		}

		switch finishReason {
		case llm.FinishReasonLength:
			// Output truncated by token limit. Persist the partial
			// assistant message (including any reasoning-only content)
			// so the user can continue the conversation.
			usage.Add(completionUsage, 0, inferenceDuration, 0)
			usage.TotalDuration = time.Since(runStart)
			a.persistMessages(ctx, dialogueID, dialogue, newMessages, usage)
			log.Debug("agent loop done: output truncated", "reason", finishReason)
			return

		case llm.FinishReasonStop:
			usage.Add(completionUsage, 0, inferenceDuration, 0)
			usage.TotalDuration = time.Since(runStart)
			a.persistMessages(ctx, dialogueID, dialogue, newMessages, usage)
			emit(ctx, ch, Event{
				Type: EventDone,
				Context: ContextSnapshot{
					TokensSent:     completionUsage.TokensSent,
					TokensReceived: completionUsage.TokensReceived,
					Window:         contextWindow,
					AutoCompactAt:  int(float64(contextWindow) * autoCompactRatio),
				},
			})
			log.Debug("agent loop done", "reason", finishReason)
			return

		case llm.FinishReasonToolCall:
			if len(assistantMsg.ToolCalls) == 0 {
				emit(ctx, ch, Event{Type: EventError,
					Error: errors.New("tool_calls finish reason but no tool calls in message")})
				usage.Add(completionUsage, 0, inferenceDuration, 0)
				usage.TotalDuration = time.Since(runStart)
				a.persistMessages(ctx, dialogueID, dialogue, newMessages, usage)
				log.Warn("agent loop done: no tool calls in response", "reason", finishReason)
				return
			}

			toolsStart := time.Now()
			emit(ctx, ch, Event{Type: EventToolsStart})

			// 1. Emit all EventToolCall events and resolve summaries upfront.
			infos = infos[:0]
			for _, call := range assistantMsg.ToolCalls {
				log.Debug("tool call",
					"name", call.Function.Name,
					"args", call.Function.Arguments,
				)
				tool, found := a.registry.Get(call.Function.Name, a.provider())
				var summary string
				if found {
					summary = tool.Summary(call.Function.Arguments)
				}
				infos = append(infos, toolCallInfo{call: call, tool: tool, found: found, summary: summary})
				emit(ctx, ch, Event{
					Type:          EventToolCall,
					ToolCallID:    call.ID,
					ToolName:      call.Function.Name,
					ToolArgs:      call.Function.Arguments,
					ToolSummary:   summary,
					ToolStartTime: time.Now(),
				})
			}

			// 2. Fan out: launch all tool executions in parallel.
			// TODO candidate
			results := make(chan executedToolCall, len(infos))
			var wg sync.WaitGroup
			for i, info := range infos {
				wg.Add(1)
				go func(i int, info toolCallInfo) {
					defer wg.Done()
					var result ToolResult
					var dur time.Duration
					if !info.found {
						log.Warn("unknown tool", "name", info.call.Function.Name)
						result = ToolResult{
							Content: fmt.Sprintf("error: unknown tool %q", info.call.Function.Name),
							IsError: true,
						}
					} else {
						toolCtx := WithCurrentModel(ctx, a.Model())
						toolCtx = WithParentToolCallID(toolCtx, info.call.ID)
						toolStart := time.Now()
						result = info.tool.Execute(toolCtx, info.call.Function.Arguments)
						dur = time.Since(toolStart)
						log.Debug("executed tool",
							"name", info.call.Function.Name,
							"error", result.IsError,
							"output", len(result.Content),
							"duration", dur,
						)
					}
					if !result.IsError {
						result.Content = truncateMiddle(result.Content, maxOutput)
					}
					results <- executedToolCall{index: i, info: info, result: result, duration: dur}
				}(i, info)
			}
			go func() { wg.Wait(); close(results) }()

			// 3. Fan in: collect results in completion order, emit EventToolResult.
			toolMsgs = slices.Grow(toolMsgs[:0], len(infos))[:len(infos)]
			clear(toolMsgs)
			compacted := false
			imageContentParts = imageContentParts[:0]
			var diagCandidates []executedToolCall
			for tr := range results {
				result := tr.result
				// Track successful apply_patch calls with touched files for auto-diagnostics.
				if !result.IsError && len(result.TouchedFiles) == 1 &&
					tr.info.call.Function.Name == "apply_patch" {
					diagCandidates = append(diagCandidates, tr)
				}

				// Handle compact: must be the sole tool call.
				if result.Compact {
					if len(assistantMsg.ToolCalls) > 1 {
						result = ToolResult{
							Content: "Compaction must be the only tool call in a response. " +
								"Call compact by itself without other tools.",
							IsError: true,
						}
					} else {
						emit(ctx, ch, Event{Type: EventCompacting})

						// Strip the trailing assistant message (the compact
						// tool call itself) — its tool result hasn't been
						// appended yet, and an unpaired tool_calls message
						// would cause an API error during summarization.
						compactD := dialogue
						compactD.Messages = messages[:len(messages)-1]
						compactedMsgs, compactErr := a.compact(ctx, ch, compactD)
						if compactErr != nil {
							result = ToolResult{
								Content: fmt.Sprintf("Compaction failed: %v. Continue without compacting.", compactErr),
								IsError: true,
							}
						} else {
							dialogue = dialoguemanager.Dialogue{ID: dialogueID, Messages: compactedMsgs, WorkspaceURI: dialogue.WorkspaceURI, Version: 1}
							newMessages = nil
							messages = append(compactedMsgs[:len(compactedMsgs):len(compactedMsgs)], resourceMsgs...)
							userMsgIdx = len(compactedMsgs) - 1
							lastAPITokensSent = 0 // force re-count with compacted messages
							compacted = true
						}
					}
				}

				// Handle clear context: reset to system prompt + tool result.
				if result.ClearContext {
					if len(assistantMsg.ToolCalls) > 1 {
						result = ToolResult{
							Content: "Clear context must be the only tool call in a response. " +
								"Call exit_plan_mode by itself without other tools.",
							IsError: true,
						}
					} else {
						clearedMsgs, clearErr := a.clearContext(ctx, ch, result.Content, dialogue)
						if clearErr != nil {
							result = ToolResult{
								Content: fmt.Sprintf("Clear context failed: %v. Continue without clearing.", clearErr),
								IsError: true,
							}
						} else {
							dialogue = dialoguemanager.Dialogue{ID: dialogueID, Messages: clearedMsgs, WorkspaceURI: dialogue.WorkspaceURI, Version: 1}
							newMessages = nil
							messages = append(clearedMsgs[:len(clearedMsgs):len(clearedMsgs)], resourceMsgs...)
							userMsgIdx = len(clearedMsgs) - 1
							lastAPITokensSent = 0 // force re-count with cleared messages
							compacted = true      // reuse flag to skip normal message append
						}
					}
				}

				emit(ctx, ch, Event{
					Type:         EventToolResult,
					ToolCallID:   tr.info.call.ID,
					ToolName:     tr.info.call.Function.Name,
					ToolOutput:   result.Content,
					ToolSummary:  tr.info.summary,
					IsError:      result.IsError,
					ToolDuration: tr.duration,
				})
				toolMsgs[tr.index] = llm.Message{
					Role:       llm.RoleTool,
					Content:    result.Content,
					ToolCallID: tr.info.call.ID,
				}
				if len(result.MultiContent) > 0 {
					imageContentParts = append(imageContentParts, result.MultiContent...)
				}
			}

			toolMsgs, assistantMsg = a.injectAutoDiagnostics(ctx, ch, messages, newMessages, assistantMsg, toolMsgs, diagCandidates, maxOutput, log)

			toolCallDuration := time.Since(toolsStart)
			usage.Add(completionUsage, len(infos), inferenceDuration, toolCallDuration)
			log.Debug("executed all tools: continuing loop",
				"duration", toolCallDuration)
			if compacted {
				// Signal a turn boundary so the TUI closes the
				// current Turn and resets streaming state before
				// the next LLM iteration begins.
				emit(ctx, ch, Event{Type: EventDone})
				continue // skip normal flow, next iteration uses compacted messages
			}

			// 4. Append tool messages in original order.
			messages = append(messages, toolMsgs...)
			newMessages = append(newMessages, toolMsgs...)

			// 5. Inject a synthetic user message for image content.
			// Neither OpenAI nor Anthropic supports images in tool-role
			// messages, so we carry the image data in a user message.
			if len(imageContentParts) > 0 {
				imgMsg := llm.Message{
					Role:         llm.RoleUser,
					MultiContent: imageContentParts,
				}
				messages = append(messages, imgMsg)
				newMessages = append(newMessages, imgMsg)
			}
			// Continue loop — next iteration feeds tool results to LLM

		default:
			// Unexpected finish reason, persist and finish
			emit(ctx, ch, Event{Type: EventError,
				Error: fmt.Errorf("unexpected finish reason: %s", finishReason)})
			usage.Add(completionUsage, 0, inferenceDuration, 0)
			usage.TotalDuration = time.Since(runStart)
			a.persistMessages(ctx, dialogueID, dialogue, newMessages, usage)
			log.Warn("agent loop done: unexpected finish reason", "reason", finishReason)
			return
		}
	}

	// Max iterations reached
	emit(ctx, ch, Event{Type: EventError,
		Error: fmt.Errorf("max iterations (%d) reached", a.config.MaxIterations)})
	usage.TotalDuration = time.Since(runStart)
	a.persistMessages(ctx, dialogueID, dialogue, newMessages, usage)
	log.Warn("agent loop done: reached max iterations", "max", a.config.MaxIterations)
}

// normalizeMessages sanitizes a message slice loaded from storage so
// it forms a valid conversation for the LLM. It fixes two classes of
// problems that arise when a previous turn was interrupted:
//
//  1. Reasoning-only assistant messages: if the model was truncated
//     before producing any text, the message has ReasoningContent but
//     empty Content. APIs reject empty text content blocks, so the
//     reasoning is moved into Content.
//
//  2. Orphaned tool calls / results: an assistant message may contain
//     tool calls whose results were never persisted (e.g. the turn was
//     interrupted before tool execution). Conversely, tool result
//     messages may lack a preceding tool call. Both cases cause API
//     errors, so orphaned tool calls are stripped from the assistant
//     message and orphaned tool results are removed entirely.
func normalizeMessages(messages []llm.Message) []llm.Message {
	// Collect the set of tool call IDs that have a matching result,
	// and the set of tool result IDs that reference an existing call.
	toolCallIDs := make(map[string]struct{})
	toolResultIDs := make(map[string]struct{})
	for i := range messages {
		for _, tc := range messages[i].ToolCalls {
			toolCallIDs[tc.ID] = struct{}{}
		}
		if messages[i].Role == llm.RoleTool && messages[i].ToolCallID != "" {
			toolResultIDs[messages[i].ToolCallID] = struct{}{}
		}
	}

	// Matched IDs are those present in both sets.
	matched := make(map[string]struct{}, len(toolCallIDs))
	for id := range toolCallIDs {
		if _, ok := toolResultIDs[id]; ok {
			matched[id] = struct{}{}
		}
	}

	n := 0
	for i := range messages {
		msg := &messages[i]

		// Strip orphaned tool calls from assistant messages.
		if len(msg.ToolCalls) > 0 {
			kept := msg.ToolCalls[:0]
			for _, tc := range msg.ToolCalls {
				if _, ok := matched[tc.ID]; ok {
					kept = append(kept, tc)
				}
			}
			msg.ToolCalls = kept
		}

		// Drop orphaned tool results.
		if msg.Role == llm.RoleTool {
			if _, ok := matched[msg.ToolCallID]; !ok {
				continue
			}
		}

		// Fix reasoning-only assistant messages.
		if msg.Role == llm.RoleAssistant &&
			msg.Content == "" &&
			msg.ReasoningContent != "" &&
			len(msg.ToolCalls) == 0 {
			msg.Content = msg.ReasoningContent
			msg.ReasoningContent = ""
		}

		// Drop assistant messages that ended up completely empty after
		// stripping (no text, no tool calls, no reasoning). These arise
		// when switching models: e.g. an OpenAI assistant message had
		// only tool calls which became orphaned. Sending an empty
		// assistant message causes Anthropic to reject with
		// "text content blocks must be non-empty".
		if msg.Role == llm.RoleAssistant &&
			msg.Content == "" &&
			msg.ReasoningContent == "" &&
			len(msg.ToolCalls) == 0 {
			continue
		}

		messages[n] = *msg
		n++
	}
	return messages[:n]
}

func (a *Agent) injectAutoDiagnostics(
	ctx context.Context,
	ch chan<- Event,
	messages []llm.Message,
	newMessages []llm.Message,
	assistantMsg llm.Message,
	toolMsgs []llm.Message,
	diagCandidates []executedToolCall,
	maxOutput int,
	log *slog.Logger,
) ([]llm.Message, llm.Message) {
	if len(diagCandidates) == 0 {
		return toolMsgs, assistantMsg
	}

	diagTool, ok := a.registry.Get("check_file_errors", a.provider())
	if !ok {
		return toolMsgs, assistantMsg
	}

	assistantIdx := len(messages) - 1
	for _, cand := range diagCandidates {
		filePath := cand.result.TouchedFiles[0]
		syntheticID := "auto-diag-" + cand.info.call.ID
		diagArgs := fmt.Sprintf(`{"path":%q}`, filePath)
		diagSummary := diagTool.Summary(diagArgs)

		syntheticCall := llm.ToolCall{
			ID:   syntheticID,
			Type: llm.ToolTypeFunction,
			Function: llm.FunctionCall{
				Name:      "check_file_errors",
				Arguments: diagArgs,
			},
		}
		assistantMsg.ToolCalls = append(assistantMsg.ToolCalls, syntheticCall)

		emit(ctx, ch, Event{
			Type:          EventToolCall,
			ToolCallID:    syntheticID,
			ToolName:      "check_file_errors",
			ToolArgs:      diagArgs,
			ToolSummary:   diagSummary,
			ToolStartTime: time.Now(),
		})

		diagStart := time.Now()
		diagResult := diagTool.Execute(
			WithParentToolCallID(WithCurrentModel(ctx, a.Model()), syntheticID),
			diagArgs,
		)
		diagDur := time.Since(diagStart)
		if !diagResult.IsError {
			diagResult.Content = truncateMiddle(diagResult.Content, maxOutput)
		}

		log.Debug("auto-diagnostics",
			"file", filePath,
			"error", diagResult.IsError,
			"output", len(diagResult.Content),
			"duration", diagDur,
		)

		emit(ctx, ch, Event{
			Type:         EventToolResult,
			ToolCallID:   syntheticID,
			ToolName:     "check_file_errors",
			ToolOutput:   diagResult.Content,
			ToolSummary:  diagSummary,
			IsError:      diagResult.IsError,
			ToolDuration: diagDur,
		})

		toolMsgs = append(toolMsgs, llm.Message{
			Role:       llm.RoleTool,
			Content:    diagResult.Content,
			ToolCallID: syntheticID,
		})
	}

	messages[assistantIdx] = assistantMsg
	newMessages[len(newMessages)-1] = assistantMsg
	return toolMsgs, assistantMsg
}

func (a *Agent) persistMessages(
	ctx context.Context, dialogueID string,
	dialogue dialoguemanager.Dialogue, newMessages []llm.Message,
	usage llm.DialogueUsage,
) {
	if len(newMessages) == 0 {
		return
	}
	// If the caller's context is already cancelled (e.g. tab closed or
	// request cancelled), use a background context with a timeout so
	// the storage operation can still complete and the conversation
	// state is not lost.
	if ctx.Err() != nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		slog.Debug("agent: persisting messages with background context", "dialogueID", dialogueID, "messages", len(newMessages))
	}
	err := a.store.Create(ctx, dialoguemanager.Dialogue{
		ID:        dialogueID,
		AgentID:   a.config.AgentID,
		Model:     a.config.Model,
		WorkspaceURI: a.config.Workspace.String(),
		SubAgent:  a.config.SubAgent,
		Messages:  newMessages,
		Usage:     usage,
	})
	if errors.Is(err, storageapi.ErrAlreadyExists) {
		err = a.store.AppendMessages(ctx, dialogue, newMessages, usage)
	}
	if err != nil {
		slog.Error("agent: persist messages", "error", err, "dialogueID", dialogueID)
	}
}

// compact summarizes the conversation, persists the compacted messages,
// and returns them. On failure it emits EventError and returns the
// error so the caller can fall through.
func (a *Agent) compact(
	ctx context.Context, ch chan<- Event,
	d dialoguemanager.Dialogue,
) ([]llm.Message, error) {
	summarizeSvc := a.svc
	if a.config.CompactSvc != nil {
		summarizeSvc = a.config.CompactSvc
	}

	compactedMsgs, archivedID, err := CompactDialogue(ctx, summarizeSvc, a.store, d)
	if err != nil {
		emit(ctx, ch, Event{Type: EventError, Error: fmt.Errorf("compact: %v", err)})
		return nil, err
	}

	emit(ctx, ch, Event{Type: EventCompacted, ArchivedDialogueID: archivedID})
	return compactedMsgs, nil
}

// extractSkillContent finds tool results containing activated skill content
// and converts them to system messages for re-injection after compaction.
func extractSkillContent(messages []llm.Message) []llm.Message {
	var result []llm.Message
	seen := make(map[string]bool)
	for _, m := range messages {
		if m.Role != llm.RoleTool || !strings.Contains(m.Content, "<skill_content ") {
			continue
		}
		if seen[m.Content] {
			continue
		}
		seen[m.Content] = true
		result = append(result, llm.Message{
			Role:    llm.RoleSystem,
			Content: m.Content,
		})
	}
	return result
}

// extractPlanContent finds messages containing an approved plan
// (wrapped in PlanContentPrefix/Suffix by clearContext) and returns them
// as system messages for re-injection after compaction. It checks both
// user messages (first compaction) and system messages (subsequent
// compactions) so the plan survives repeated summarisation.
func extractPlanContent(messages []llm.Message) []llm.Message {
	var result []llm.Message
	for _, m := range messages {
		if !strings.HasPrefix(m.Content, PlanContentPrefix) {
			continue
		}
		if m.Role != llm.RoleUser && m.Role != llm.RoleSystem {
			continue
		}
		result = append(result, llm.Message{
			Role:    llm.RoleSystem,
			Content: m.Content,
		})
		break // only one plan per conversation
	}
	return result
}

// clearContext replaces the conversation with just a system prompt and
// a user message (typically the approved plan), archives the old messages,
// and overwrites the current dialogue in-place. The dialogue ID never changes.
func (a *Agent) clearContext(
	ctx context.Context, ch chan<- Event,
	content string,
	d dialoguemanager.Dialogue,
) ([]llm.Message, error) {
	archivedID, err := NextArchivedID(ctx, a.store, d.ID)
	if err != nil {
		emit(ctx, ch, Event{Type: EventError, Error: fmt.Errorf("clear context: archive: %v", err)})
		return nil, err
	}

	// Re-inject the system prompt from the original messages.
	var systemPrompt string
	if len(d.Messages) > 0 && d.Messages[0].Role == llm.RoleSystem {
		systemPrompt = d.Messages[0].Content
	}

	clearedMsgs := []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
		{Role: llm.RoleUser, Content: PlanContentPrefix + content + PlanContentSuffix},
	}

	if err := a.store.ArchiveAndReplace(ctx, dialoguemanager.ArchiveAndReplaceParams{
		Dialogue:           d,
		ArchivedDialogueID: archivedID,
		Messages:           clearedMsgs,
	}); err != nil {
		emit(ctx, ch, Event{Type: EventError, Error: fmt.Errorf("clear context: %v", err)})
		return nil, err
	}

	emit(ctx, ch, Event{Type: EventCompacted, ArchivedDialogueID: archivedID})
	return clearedMsgs, nil
}

// CompactSummaryPrefix is prepended to the LLM-generated summary when
// building the compacted user message. It frames the summary as a
// continuation from a prior context window so the model can resume
// seamlessly. Placing the summary in a user message (rather than
// assistant) avoids assistant prefill, which some providers reject.
const CompactSummaryPrefix = "This session is being continued from a previous conversation that ran out of context. " +
	"The summary below covers the earlier portion of the conversation.\n\n"

// PlanContentPrefix marks a user message as an approved plan that must
// survive compaction. clearContext wraps the plan with this prefix so
// that extractPlanContent can recognise and re-inject it.
const PlanContentPrefix = "<approved_plan>\n"

// PlanContentSuffix closes the approved plan marker.
const PlanContentSuffix = "\n</approved_plan>"

// ArchivedID returns the base archive dialogue ID for the given dialogue.
// It strips any existing "-archived" (with optional numeric suffix) to avoid accumulation.
func ArchivedID(dialogueID string) string {
	dialogueID = strings.TrimSuffix(dialogueID, "-archived")
	dialogueID = strings.TrimSuffix(dialogueID, "-compacted")
	dialogueID = strings.TrimSuffix(dialogueID, "-cleared")
	return dialogueID + "-archived"
}

// NextArchivedID finds the next available archive ID by probing the store.
// It returns "<base>-archived" if that slot is free, otherwise
// "<base>-archived-2", "<base>-archived-3", etc.
func NextArchivedID(ctx context.Context, store dialoguemanager.Store, dialogueID string) (string, error) {
	base := ArchivedID(dialogueID)
	if _, err := store.Get(ctx, base); errors.Is(err, storageapi.ErrNotFound) {
		return base, nil
	} else if err != nil {
		return "", err
	}
	for i := 2; ; i++ {
		candidate := base + "-" + strconv.Itoa(i)
		if _, err := store.Get(ctx, candidate); errors.Is(err, storageapi.ErrNotFound) {
			return candidate, nil
		} else if err != nil {
			return "", err
		}
	}
}

var (
	reAnalysis   = regexp.MustCompile(`(?s)<analysis>.*?</analysis>`)
	reSummary    = regexp.MustCompile(`(?s)<summary>(.*?)</summary>`)
	reBlankLines = regexp.MustCompile(`\n\n+`)
)

// SummarizePrompt is the prompt sent to the LLM when compacting a
// conversation. It instructs the model to produce a structured 9-section
// summary wrapped in <summary> tags, with an optional <analysis>
// scratchpad that is stripped before use.
const SummarizePrompt = `Your task is to create a detailed summary of the conversation so far, paying close attention to the user's explicit requests and your previous actions.
This summary should be thorough in capturing technical details, code patterns, and architectural decisions that would be essential for continuing development work without losing context.

Before providing your final summary, wrap your analysis in <analysis> tags to organize your thoughts and ensure you've covered all necessary points. In your analysis process:
1. Chronologically analyze each message and section of the conversation. For each section thoroughly identify:
   - The user's explicit requests and intents
   - Your approach to addressing the user's requests
   - Key decisions, technical concepts and code patterns
   - Specific details like: file names, full code snippets, function signatures, file edits
   - Errors that you ran into and how you fixed them
   - Pay special attention to specific user feedback that you received, especially if the user told you to do something differently.
2. Double-check for technical accuracy and completeness, addressing each required element thoroughly.

Your summary should include the following sections:

1. Primary Request and Intent: Capture all of the user's explicit requests and intents in detail
2. Key Technical Concepts: List all important technical concepts, technologies, and frameworks discussed.
3. Files and Code Sections: Enumerate specific files and code sections examined, modified, or created. Pay special attention to the most recent messages and include full code snippets where applicable and include a summary of why this file read or edit is important.
4. Errors and fixes: List all errors that you ran into, and how you fixed them. Pay special attention to specific user feedback that you received, especially if the user told you to do something differently.
5. Problem Solving: Document problems solved and any ongoing troubleshooting efforts.
6. All user messages: List ALL user messages that are not tool results. These are critical for understanding the users' feedback and changing intent.
7. Pending Tasks: Outline any pending tasks that you have explicitly been asked to work on.
8. Current Work: Describe in detail precisely what was being worked on immediately before this summary request, paying special attention to the most recent messages from both user and assistant. Include file names and code snippets where applicable.
9. Optional Next Step: List the next step that you will take that is related to the most recent work you were doing. IMPORTANT: ensure that this step is DIRECTLY in line with the user's most recent explicit requests, and the task you were working on immediately before this summary request. If your last task was concluded, then only list next steps if they are explicitly in line with the users request. Do not start on tangential requests or really old requests that were already completed without confirming with the user first.
                       If there is a next step, include direct quotes from the most recent conversation showing exactly what task you were working on and where you left off. This should be verbatim to ensure there's no drift in task interpretation.

Please provide your output in the following format:

<analysis>
[Your thought process, checking each required element]
</analysis>

<summary>
[Your summary here, with all 9 sections]
</summary>

IMPORTANT: Do NOT use any tools. You MUST respond with ONLY the <summary>...</summary> block as your text output.`

// cleanSummary strips the <analysis> scratchpad from the raw LLM output
// and extracts the <summary> content. If no <summary> tags are found the
// text is returned as-is (plain-text fallback).
func cleanSummary(raw string) string {
	text := reAnalysis.ReplaceAllString(raw, "")
	if m := reSummary.FindStringSubmatch(text); len(m) >= 2 {
		inner := strings.TrimSpace(m[1])
		text = reSummary.ReplaceAllString(text, "Summary:\n"+inner)
	}
	text = reBlankLines.ReplaceAllString(text, "\n")
	return strings.TrimSpace(text)
}

// Summarize sends the given messages to the LLM and asks it to produce
// a structured summary. It returns the cleaned summary text.
func Summarize(ctx context.Context, svc llm.Service, messages []llm.Message) (string, error) {
	messages = normalizeMessages(slices.Clone(messages))

	prompt := llm.Message{
		Role:    llm.RoleUser,
		Content: SummarizePrompt,
	}
	summaryReq := llm.Request{
		Messages: append(messages, prompt),
	}

	it, err := svc.CreateCompletion(ctx, summaryReq)
	if err != nil {
		return "", err
	}
	defer it.Close() //nolint:errcheck

	var sb strings.Builder
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		if ev.Type == llm.EventTextDelta {
			sb.WriteString(ev.Text)
		}
		if ev.Type == llm.EventStreamError {
			return "", ev.Error
		}
	}
	if err := it.Err(); err != nil {
		return "", err
	}
	summary := cleanSummary(sb.String())
	if summary == "" {
		return "", fmt.Errorf("LLM returned an empty summary")
	}
	return summary, nil
}

// CompactDialogue summarizes the conversation, archives the old messages
// under a unique archived ID, and overwrites the current dialogue in-place
// with the compacted messages. The dialogue ID never changes. It returns
// both the compacted messages and the archived dialogue ID.
func CompactDialogue(
	ctx context.Context,
	svc llm.Service,
	store dialoguemanager.Store,
	d dialoguemanager.Dialogue,
) (compactedMsgs []llm.Message, archivedDialogueID string, err error) {
	summaryText, err := Summarize(ctx, svc, d.Messages)
	if err != nil {
		return nil, "", err
	}

	archivedID, err := NextArchivedID(ctx, store, d.ID)
	if err != nil {
		return nil, "", fmt.Errorf("archive: %w", err)
	}

	// Re-inject the system prompt from the original messages.
	var systemPrompt string
	if len(d.Messages) > 0 && d.Messages[0].Role == llm.RoleSystem {
		systemPrompt = d.Messages[0].Content
	}

	compactedMsgs = []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
		{Role: llm.RoleUser, Content: CompactSummaryPrefix + summaryText},
	}

	// Preserve plan and skill content from the old messages.
	var preserved []llm.Message
	preserved = append(preserved, extractPlanContent(d.Messages)...)
	preserved = append(preserved, extractSkillContent(d.Messages)...)
	if len(preserved) > 0 {
		compactedMsgs = slices.Insert(compactedMsgs, 1, preserved...)
	}

	if err := store.ArchiveAndReplace(ctx, dialoguemanager.ArchiveAndReplaceParams{
		Dialogue:           d,
		ArchivedDialogueID: archivedID,
		Messages:           compactedMsgs,
	}); err != nil {
		return nil, "", fmt.Errorf("persist: %w", err)
	}

	return compactedMsgs, archivedID, nil
}

func emit(ctx context.Context, ch chan<- Event, ev Event) {
	select {
	case ch <- ev:
	case <-ctx.Done():
	}
}

// channelIterator adapts a channel to iterator.Iterator[Event].
type channelIterator struct {
	ch  <-chan Event
	cur Event
	err error
}

func (c *channelIterator) Next(ctx context.Context) (Event, bool) {
	select {
	case ev, ok := <-c.ch:
		if !ok {
			return Event{}, false
		}
		c.cur = ev
		if ev.Type == EventError {
			c.err = ev.Error
		}
		return ev, true
	case <-ctx.Done():
		c.err = ctx.Err()
		return Event{}, false
	}
}

func (c *channelIterator) Err() error {
	return c.err
}

func (c *channelIterator) Close() error {
	return nil
}

// buildToolCallMessages converts pre-computed ToolCallResults into the
// assistant + tool message pairs that would have been produced by
// actual tool executions. The assistant message contains all tool
// calls; each result follows as a separate tool-role message.
func buildToolCallMessages(results []ToolCallResult) []llm.Message {
	calls := make([]llm.ToolCall, len(results))
	for i, r := range results {
		calls[i] = llm.ToolCall{
			ID:   fmt.Sprintf("precall-%d", i),
			Type: llm.ToolTypeFunction,
			Function: llm.FunctionCall{
				Name:      r.ToolName,
				Arguments: r.Arguments,
			},
		}
	}

	msgs := make([]llm.Message, 0, 1+len(results))
	msgs = append(msgs, llm.Message{
		Role:      llm.RoleAssistant,
		ToolCalls: calls,
	})
	for i, r := range results {
		msgs = append(msgs, llm.Message{
			Role:       llm.RoleTool,
			Content:    r.Content,
			ToolCallID: calls[i].ID,
		})
	}
	return msgs
}

// estimateToolDefTokens approximates the token overhead of tool
// definitions without JSON serialization. It walks each tool's name,
// description, and parameter schema to estimate the JSON byte length,
// then divides by 4 (a conservative chars-per-token ratio for
// structured text). Zero allocations.
func estimateToolDefTokens(tools []llm.Tool) int {
	var n int
	for i := range tools {
		f := &tools[i].Function
		// JSON envelope: {"type":"...","function":{"name":"...","description":"...","parameters":...}}
		n += len(tools[i].Type) + len(f.Name) + len(f.Description) + 60
		if f.Parameters != nil {
			n += estimateJSONSize(f.Parameters)
		}
	}
	return n / 4
}

// estimateJSONSize approximates the JSON byte length of a value
// without serializing it. Handles the types that encoding/json
// produces when unmarshalling into any (map, slice, string, float64, bool, nil).
func estimateJSONSize(v any) int {
	switch v := v.(type) {
	case map[string]any:
		n := 2 // {}
		for k, val := range v {
			n += len(k) + 4 + estimateJSONSize(val) // "key":val,
		}
		return n
	case []any:
		n := 2 // []
		for _, val := range v {
			n += estimateJSONSize(val) + 1 // val,
		}
		return n
	case string:
		return len(v) + 2
	case bool:
		return 5
	case float64:
		return 10
	default:
		return 4 // null
	}
}
