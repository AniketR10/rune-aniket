// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.

package extension

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"

	"unstable.build/go-tui/cmd/rune-agent/agent/skills"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguetui"
	"unstable.build/go-tui/cmd/rune-agent/llm/llmtest"
)

func mustURI(t *testing.T, s string) workspaceapi.URI {
	t.Helper()
	u, err := workspaceapi.ParseURI(s)
	require.NoError(t, err)
	return u
}

func TestDialogueIDFromURI(t *testing.T) {
	tests := []struct {
		name   string
		uri    string
		wantID string
		wantOK bool
	}{
		{"valid", "rune-agent://openai_gpt-5/rolling-fox", "rolling-fox", true},
		{"escaped model", "rune-agent://hf.co_org_model/quiet-owl", "quiet-owl", true},
		{"no id", "rune-agent://openai_gpt-5/", "", false},
		{"missing path", "rune-agent://openai_gpt-5", "", false},
		{"other scheme", "file:///tmp/foo", "", false},
		{"empty", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var uri workspaceapi.URI
			if tc.uri != "" {
				uri = mustURI(t, tc.uri)
			}
			id, ok := dialogueIDFromURI(uri)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantID, id)
		})
	}
}

func TestRouteChatCommandDelivers(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		args     []string
		wantName string
		wantArgs []string
	}{
		{"model", commandModel, []string{"openai/gpt-5"}, "model", []string{"openai/gpt-5"}},
		{"effort", commandEffort, []string{"high"}, "effort", []string{"high"}},
		{"maxtokens", commandMaxTokens, []string{"4096"}, "max_tokens", []string{"4096"}},
		{"skill", commandSkill, []string{"review", "foo"}, "review", []string{"foo"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			ch := make(chan dialoguetui.MessageEvent, 1)
			h := &aiEditorHandler{ctx: ctx}
			h.openChatTx.Store("rolling-fox", (chan<- dialoguetui.MessageEvent)(ch))

			cmd := textapi.Command{
				Name: tc.command,
				Args: tc.args,
				URI:  mustURI(t, "rune-agent://openai_gpt-5/rolling-fox"),
			}
			require.NoError(t, h.routeChatCommand(cmd))

			select {
			case ev := <-ch:
				assert.Equal(t, dialoguetui.MessageEventCommand, ev.Type)
				assert.Equal(t, tc.wantName, ev.CommandName)
				assert.Equal(t, tc.wantArgs, ev.CommandArgs)
			case <-time.After(2 * time.Second):
				t.Fatal("timed out waiting for injected command event")
			}
		})
	}
}

func TestRouteChatCommandErrors(t *testing.T) {
	ctx := context.Background()
	h := &aiEditorHandler{ctx: ctx}

	// No focused chat tab.
	err := h.routeChatCommand(textapi.Command{Name: commandModel})
	require.Error(t, err)

	// Chat URI that is not an open chat.
	err = h.routeChatCommand(textapi.Command{
		Name: commandModel,
		URI:  mustURI(t, "rune-agent://openai_gpt-5/missing-chat"),
	})
	require.Error(t, err)

	// skill without a name.
	h.openChatTx.Store("rolling-fox",
		(chan<- dialoguetui.MessageEvent)(make(chan dialoguetui.MessageEvent, 1)))
	err = h.routeChatCommand(textapi.Command{
		Name: commandSkill,
		URI:  mustURI(t, "rune-agent://openai_gpt-5/rolling-fox"),
	})
	require.Error(t, err)
}

func TestCompleteChatPromptCommands(t *testing.T) {
	svc := llmtest.New([]llmapi.ModelEntry{
		{Provider: "openai", Name: "gpt-5"},
		{Provider: "codex", Name: "gpt-5"},
	})
	skillReg := skills.NewRegistry(nopFileSystem{}, dirURI(""), nil, nil)
	h := &aiEditorHandler{llmSvc: svc, skillRegistry: skillReg}
	ctx := context.Background()

	t.Run("model", func(t *testing.T) {
		got := completeToSlice(t, ctx, h, commandModel)
		assert.Equal(t, []string{"openai/gpt-5", "codex/gpt-5"}, got)
	})
	t.Run("effort", func(t *testing.T) {
		got := completeToSlice(t, ctx, h, commandEffort)
		assert.Equal(t, []string{
			"none", "minimal", "low", "medium", "high", "xhigh", "max",
		}, got)
	})
	t.Run("skill", func(t *testing.T) {
		got := completeToSlice(t, ctx, h, commandSkill)
		assert.Equal(t, []string{"explore", "plan"}, got)
	})
	t.Run("maxtokens", func(t *testing.T) {
		got := completeToSlice(t, ctx, h, commandMaxTokens)
		assert.Empty(t, got)
	})
}

func completeToSlice(
	t *testing.T, ctx context.Context, h *aiEditorHandler, name string,
) []string {
	t.Helper()
	it, err := h.Complete(ctx, name, nil)
	require.NoError(t, err)
	got, err := iterator.ToSlice(ctx, it)
	require.NoError(t, err)
	return got
}
