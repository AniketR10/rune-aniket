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
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
	"unstable.build/go-tui/cmd/rune-agent/hooks"
)

// ServiceFactory resolves an LLM service for the given model name.
// It returns the service, the fully populated ModelEntry (carrying
// the resolved Provider and ContextWindow used by the agent loop),
// and any error.
type ServiceFactory func(model string) (llmapi.Service, llmapi.ModelEntry, error)

// GoroutineSpawner is a per-session Spawner that runs
// sub-agents as goroutines. Each chat session creates its
// own GoroutineSpawner.
type GoroutineSpawner struct {
	store               dialoguemanager.Store
	serviceFactory      ServiceFactory
	config              *Cfg
	registry            *Registry
	skillRegistry       *skills.SkillRegistry
	memory              MemoryRecaller
	projectInstructions string
	sessionKey          string
	agentID             string
	workspace           workspaceapi.URI
	hookRunner          *hooks.Runner
	prompter            Prompter

	// GenerateDialogueID, when non-nil, replaces the default
	// petname generator for child dialogue IDs. Intended for testing.
	GenerateDialogueID func(ctx context.Context, agentID string) string
}

// NewGoroutineSpawner creates a GoroutineSpawner for the
// given session. Call SetRegistry before any Run calls
// to provide the tool registry (see SetRegistry).
//
// prompter is mandatory; it is propagated to sub-agents via
// agent.Config so per-call tool decisions (e.g. the grep guard)
// can ask the user. Pass a no-op prompter when sub-agents must
// never block on input. Panics if prompter is nil.
func NewGoroutineSpawner(
	store dialoguemanager.Store, serviceFactory ServiceFactory,
	config *Cfg,
	skillRegistry *skills.SkillRegistry,
	memory MemoryRecaller,
	projectInstructions string,
	sessionKey, agentID string,
	workspace workspaceapi.URI,
	prompter Prompter,
) *GoroutineSpawner {
	if prompter == nil {
		panic("agent.NewGoroutineSpawner: prompter must not be nil")
	}
	return &GoroutineSpawner{
		store:               store,
		serviceFactory:      serviceFactory,
		config:              config,
		skillRegistry:       skillRegistry,
		memory:              memory,
		projectInstructions: projectInstructions,
		sessionKey:          sessionKey,
		agentID:             agentID,
		workspace:           workspace,
		prompter:            prompter,
	}
}

// SetRegistry sets the tool registry for sub-agents. Must be
// called before any Run calls. This is separate from
// construction because the registry may contain tools (like
// the skill tool) that reference the spawner itself.
func (s *GoroutineSpawner) SetRegistry(r *Registry) {
	s.registry = r
}

// SetHooks installs the hook runner that child sub-agents inherit.
// Must be called before any Run calls; nil is a no-op.
func (s *GoroutineSpawner) SetHooks(r *hooks.Runner) {
	s.hookRunner = r
}

// Run validates the request, creates a sub-agent, and returns
// a RunHandle whose Events iterator delivers the sub-agent's
// events. Closing the iterator cancels the sub-agent and
// optionally deletes its dialogue (Cleanup=="delete").
// No goroutines are launched inside the spawner — the caller
// decides how to consume the iterator.
func (s *GoroutineSpawner) Run(
	ctx context.Context, req RunRequest,
) (RunHandle, error) {
	agentID := req.AgentID
	if agentID == "" {
		agentID = s.agentID
	}
	def, ok := s.config.Get(agentID)
	skillBased := false
	if !ok {
		// Fallback: check skill registry for agent-type skill
		// (case-insensitive). This lets the LLM call the agent
		// tool with subagent_type="Plan" and have it resolve to
		// the "plan" skill transparently.
		skill, skillOK := s.skillRegistry.GetFold(agentID)
		if !skillOK || skill.Type != "agent" {
			return RunHandle{}, fmt.Errorf("no agent or agent skill named %q exists", agentID)
		}
		def = Definition{
			ID:           skill.Name,
			Name:         skill.Description,
			SystemPrompt: skill.Body,
			Model:        skill.Model,
		}
		if skill.AllowedTools != "" && len(req.AllowedTools) == 0 {
			req.AllowedTools = strings.Fields(skill.AllowedTools)
		}
		skillBased = true
		agentID = skill.Name
	}

	if !skillBased && !s.isAllowed(agentID) {
		return RunHandle{}, fmt.Errorf(
			"no agent or agent skill named %q exists",
			agentID,
		)
	}

	if req.Cleanup != "" && req.Cleanup != "delete" && req.Cleanup != "keep" {
		return RunHandle{}, fmt.Errorf(
			"invalid cleanup value %q: must be \"delete\" or \"keep\"",
			req.Cleanup,
		)
	}

	model := req.Model
	if model == "" {
		model = def.Model
	}
	if model == "" {
		model = CurrentModel(ctx)
	}

	svc, entry, err := s.serviceFactory(model)
	if err != nil {
		return RunHandle{}, fmt.Errorf("create service for model %q: %w", model, err)
	}

	var dialogueID string
	if s.GenerateDialogueID != nil {
		dialogueID = s.GenerateDialogueID(ctx, agentID)
	} else {
		prefix := "sub-agent-" + agentID + "-"
		dialogueID = dialoguemanager.GenerateUniqueID(ctx, s.store, prefix)
	}
	sessionKey := dialogueID
	label := req.Label
	if label == "" {
		label = sessionKey
	}

	registry := s.registry
	if registry == nil {
		registry = NewRegistry()
	}
	if len(req.AllowedTools) > 0 {
		registry = registry.WithFilteredTools(req.AllowedTools)
	}
	prompt := req.SystemPrompt
	if prompt == "" {
		prompt = def.SystemPrompt
	}
	if prompt == "" {
		prompt = "You are a sub-agent. Complete the task."
	}
	ag := NewAgent(svc, registry, s.skillRegistry, s.store, s.memory, Config{
		SystemPrompt:        prompt,
		ProjectInstructions: s.projectInstructions,
		SessionKey:          sessionKey,
		AgentID:             agentID,
		Model:               entry,
		SubAgent:            true,
		Workspace:           s.workspace,
		Prompter:            s.prompter,
	})

	// Sub-agent runs inherit the caller's cancellation but have no
	// artificial deadline: a tool call may block indefinitely on user
	// input (e.g. ideauthorizer permission prompts), and a deadline
	// would cause those to fail spuriously. Cancellation still flows
	// from session teardown and from the parent agent closing the
	// returned iterator.
	runCtx, cancel := context.WithCancel(ctx)

	// If the caller supplied initial messages (e.g. a snapshot of the
	// parent dialogue), pre-create the child dialogue with the child's
	// own system prompt followed by those seed messages. Agent.Run will
	// then load this dialogue and append the current task message. The
	// dialogue's metadata mirrors what Agent.Run would write for a
	// freshly-created sub-agent dialogue (same WorkspaceURI source,
	// same SubAgent flag, same AgentID/Model).
	if len(req.InitialMessages) > 0 {
		seed := make([]llmapi.Message, 0, 1+len(req.InitialMessages))
		seed = append(seed, llmapi.Message{Role: llmapi.RoleSystem, Content: prompt})
		seed = append(seed, req.InitialMessages...)
		if createErr := s.store.Create(runCtx, dialoguemanager.Dialogue{
			ID:           dialogueID,
			AgentID:      agentID,
			Model:        model,
			WorkspaceURI: s.workspace.String(),
			SubAgent:     true,
			Messages:     seed,
		}); createErr != nil {
			slog.Warn("spawner: seed child dialogue",
				"error", createErr, "dialogueID", dialogueID)
		}
	}

	inner := ag.Run(runCtx, dialogueID, req.Message)
	it := &spawnerIterator{
		inner:      inner,
		cancel:     cancel,
		store:      s.store,
		dialogueID: dialogueID,
		cleanup:    req.Cleanup,
		hooks:      s.hookRunner,
		workspace:  s.workspace,
	}

	return RunHandle{
		SessionKey: sessionKey,
		DialogueID: dialogueID,
		Label:      label,
		Events:     it,
	}, nil
}

// ListAgents returns the agents that this session's agent
// is permitted to spawn.
func (s *GoroutineSpawner) ListAgents() []AgentSummary {
	defs := s.config.AllowedAgents(s.agentID)
	result := make([]AgentSummary, len(defs))
	for i, d := range defs {
		result[i] = AgentSummary{ID: d.ID, Name: d.Name}
	}
	return result
}

func (s *GoroutineSpawner) isAllowed(targetID string) bool {
	requester, ok := s.config.Get(s.agentID)
	if !ok {
		return false
	}
	if requester.AllowAny {
		return true
	}
	return slices.Contains(requester.AllowSpawn, targetID)
}

// spawnerIterator wraps an event iterator and, on Close,
// cancels the sub-agent's context and optionally deletes
// its dialogue.
type spawnerIterator struct {
	inner      iterator.Iterator[Event]
	cancel     context.CancelFunc
	store      dialoguemanager.Store
	dialogueID string
	cleanup    string
	hooks      *hooks.Runner
	workspace  workspaceapi.URI
}

func (s *spawnerIterator) Next(ctx context.Context) (Event, bool) {
	return s.inner.Next(ctx)
}

func (s *spawnerIterator) Err() error {
	return s.inner.Err()
}

func (s *spawnerIterator) Close() error {
	s.cancel()
	err := s.inner.Close()
	// SessionEnd hook (reason=subagent): fire-and-forget once the
	// child agent loop has exited.
	s.hooks.Run(context.Background(), hooks.Payload{
		SessionID:     s.dialogueID,
		Cwd:           s.workspace,
		HookEventName: hooks.EventSessionEnd,
		Reason:        "subagent",
	})
	if s.cleanup == "delete" {
		delCtx, delCancel := context.WithTimeout(
			context.Background(), 10*time.Second,
		)
		defer delCancel()
		if delErr := s.store.Delete(delCtx, s.dialogueID); delErr != nil {
			slog.Warn("spawner: cleanup dialogue",
				"error", delErr,
				"dialogueID", s.dialogueID)
		}
	}
	return err
}
