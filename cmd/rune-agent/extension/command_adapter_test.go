// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package extension

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"

	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
	"unstable.build/go-tui/cmd/rune-agent/llm/llmtest"
)

// captureCommandHandler records the last repl.Command it received and
// yields no output components.
type captureCommandHandler struct {
	last repl.Command
}

func (c *captureCommandHandler) HandleCommand(
	_ context.Context, cmd repl.Command, _ repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	c.last = cmd
	return iterator.FromSlice[component.Responsive](nil), nil
}

func (c *captureCommandHandler) Complete(
	_ context.Context, _ string, _ []string,
) (iterator.Iterator[string], error) {
	return iterator.FromSlice[string](nil), nil
}

func newCaptureAdapter(h repl.CommandHandler) *commandAdapter {
	return &commandAdapter{
		handler:       h,
		dialogueID:    "rolling-fox",
		skillRegistry: skills.NewRegistry(nopFileSystem{}, dirURI(""), nil, nil),
	}
}

// Chat commands act on the open chat only: each must dispatch the
// equivalent shell subcommand with the adapter's own dialogue id and no
// caller-supplied positional id.
func TestCommandAdapterScopesToOpenChat(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantName string
		wantArgs []string
	}{
		{"history", nil, "chats", []string{"show", "rolling-fox"}},
		{"log", nil, "chats", []string{"log", "rolling-fox"}},
		{"fork", nil, "chats", []string{"fork", "rolling-fox"}},
		{"export", nil, "chats", []string{"export", "rolling-fox"}},
		{"export", []string{"--audit"}, "chats", []string{"export", "--audit", "rolling-fox"}},
	}
	for _, tc := range tests {
		t.Run(tc.name+strings.Join(tc.args, ""), func(t *testing.T) {
			h := &captureCommandHandler{}
			a := newCaptureAdapter(h)
			_, err := a.HandleCommand(context.Background(), tc.name, tc.args)
			require.NoError(t, err)
			assert.Equal(t, tc.wantName, h.last.Name)
			assert.Equal(t, tc.wantArgs, h.last.Args)
		})
	}
}

// compact returns a lazy iterator that dispatches against the open
// chat; draining it issues "chats compact <dialogueID>".
func TestCommandAdapterCompactScopesToOpenChat(t *testing.T) {
	h := &captureCommandHandler{}
	a := newCaptureAdapter(h)
	res, err := a.HandleCommand(context.Background(), "compact", nil)
	require.NoError(t, err)
	require.NotNil(t, res.Display)
	_, _ = res.Display.Next(context.Background())
	assert.Equal(t, "chats", h.last.Name)
	assert.Equal(t, []string{"compact", "rolling-fox"}, h.last.Args)
}

// Any positional dialogue id is rejected; chat commands no longer
// target other dialogues.
func TestCommandAdapterRejectsPositionalID(t *testing.T) {
	for _, name := range []string{"clear", "history", "compact", "export", "log", "fork"} {
		t.Run(name, func(t *testing.T) {
			h := &captureCommandHandler{}
			a := newCaptureAdapter(h)
			_, err := a.HandleCommand(context.Background(), name, []string{"other-chat"})
			require.Error(t, err)
		})
	}
}

// Bare names must not appear in /model completions: two providers
// can ship the same Name (e.g. openai/gpt-5.5 vs codex/gpt-5.5),
// and a bare candidate would silently dispatch to whichever provider
// Models() iterates first.
func TestCommandAdapterModelCompleter(t *testing.T) {
	svc := llmtest.New([]llmapi.ModelEntry{
		{Provider: "openai", Name: "gpt-5.5"},
		{Provider: "codex", Name: "gpt-5.5"},
		{Provider: "openai", Name: "gpt-4o"},
	})
	a := &commandAdapter{llmSvc: svc}
	ctx := context.Background()
	it, err := a.Complete(ctx, "model", nil)
	require.NoError(t, err)
	got, err := iterator.ToSlice(ctx, it)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"openai/gpt-5.5",
		"codex/gpt-5.5",
		"openai/gpt-4o",
	}, got)
}

// aliasResolvingService resolves the bare name "default" to a
// fully-qualified entry, mirroring how the host router exposes alias
// resolution through GetModel with an empty Provider.
type aliasResolvingService struct {
	*llmtest.Service
	target llmapi.ModelEntry
}

func (s *aliasResolvingService) GetModel(
	_ context.Context, model llmapi.ModelEntry,
) (llmapi.ModelEntry, error) {
	if model.Provider == "" && model.Name == "default" {
		return s.target, nil
	}
	return s.Service.GetModel(context.Background(), model)
}

// /model with no args must show the resolved provider/model, not the
// bare alias the session was created with (e.g. "default").
func TestCommandAdapterModelResolvesAliasLabel(t *testing.T) {
	target := llmapi.ModelEntry{Provider: "openai", Name: "gpt-5.5"}
	svc := &aliasResolvingService{
		Service: llmtest.New([]llmapi.ModelEntry{target}),
		target:  target,
	}
	a := &commandAdapter{llmSvc: svc, currentModel: "default"}

	assert.Equal(t, "openai/gpt-5.5", a.resolvedModelLabel(context.Background()))
}

// Slash-command preloads ship the skill body as a separate system
// message, which the model reliably misses. formatSkillMessage must
// include an inline cue telling the model to invoke the skill tool so
// the body actually gets attended to.
func TestFormatSkillMessageIncludesSkillToolHint(t *testing.T) {
	skill := skills.Skill{Name: "perplexity-search"}

	msg := formatSkillMessage(skill, "pico go to line")

	assert.Contains(t, msg, "<command-message>perplexity-search</command-message>")
	assert.Contains(t, msg, "<command-name>/perplexity-search</command-name>")
	assert.Contains(t, msg, "pico go to line")
	assert.Contains(t, msg, "<command-hint>")
	assert.Contains(t, msg, "</command-hint>")
	assert.Contains(t, msg, "perplexity-search")
	// The hint must mention the skill tool by name so the model knows
	// what action to take.
	hintStart := strings.Index(msg, "<command-hint>")
	hintEnd := strings.Index(msg, "</command-hint>")
	require.Greater(t, hintEnd, hintStart)
	hint := msg[hintStart+len("<command-hint>") : hintEnd]
	assert.Contains(t, hint, "skill")
}

// parseStoredCommandMessage must continue to recover the slash-command
// name and args from history even when the message carries the new
// command-hint envelope.
func TestParseStoredCommandMessageRoundTripsWithHint(t *testing.T) {
	tests := []struct {
		name     string
		skill    skills.Skill
		args     string
		expected string
	}{
		{
			name:     "no args",
			skill:    skills.Skill{Name: "clear"},
			args:     "",
			expected: "/clear",
		},
		{
			name:     "single-line args",
			skill:    skills.Skill{Name: "perplexity-search"},
			args:     "pico go to line",
			expected: "/perplexity-search pico go to line",
		},
		{
			name:     "multi-line args",
			skill:    skills.Skill{Name: "issue-create"},
			args:     "line one\nline two",
			expected: "/issue-create\nline one\nline two",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := formatSkillMessage(tc.skill, tc.args)
			got, ok := parseStoredCommandMessage(msg)
			require.True(t, ok, "parse should succeed for %q", msg)
			assert.Equal(t, tc.expected, got)
		})
	}
}

// Same hazard as TestCommandAdapterModelCompleter, for :chat / :query.
func TestHandlerModelCompleter(t *testing.T) {
	svc := llmtest.New([]llmapi.ModelEntry{
		{Provider: "openai", Name: "gpt-5.5"},
		{Provider: "codex", Name: "gpt-5.5"},
		{Provider: "openai", Name: "gpt-4o"},
	})
	h := &aiEditorHandler{llmSvc: svc}
	ctx := context.Background()
	it, err := h.completeWithModelsIterator(ctx)
	require.NoError(t, err)
	got, err := iterator.ToSlice(ctx, it)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"openai/gpt-5.5",
		"codex/gpt-5.5",
		"openai/gpt-4o",
	}, got)
}
