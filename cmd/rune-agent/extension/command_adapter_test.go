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
	"github.com/unstablebuild/rune-go-sdk/iterator"

	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
	"unstable.build/go-tui/cmd/rune-agent/llm/llmtest"
)

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
