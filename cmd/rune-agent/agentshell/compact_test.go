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

package agentshell

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"

	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
	"unstable.build/go-tui/cmd/rune-agent/llm/llmtest"
)

type compactAliasService struct {
	*llmtest.Service
	target llmapi.ModelEntry
}

func (s *compactAliasService) GetModel(
	ctx context.Context, model llmapi.ModelEntry,
) (llmapi.ModelEntry, error) {
	if model.Provider == "" && model.Name == "compact" {
		return s.target, nil
	}
	return s.Service.GetModel(ctx, model)
}

type compactDialogueStore struct {
	dialogue dialoguemanager.Dialogue
}

func (s *compactDialogueStore) Health(context.Context) error { return nil }
func (s *compactDialogueStore) Create(_ context.Context, d dialoguemanager.Dialogue) error {
	s.dialogue = d
	return nil
}
func (s *compactDialogueStore) Get(_ context.Context, id string) (dialoguemanager.Dialogue, error) {
	if id != s.dialogue.ID {
		return dialoguemanager.Dialogue{}, storageapi.ErrNotFound
	}
	return s.dialogue, nil
}
func (s *compactDialogueStore) Delete(context.Context, string) error { return nil }
func (s *compactDialogueStore) AppendMessages(
	context.Context, dialoguemanager.Dialogue, []llmapi.Message, llmapi.DialogueUsage,
) error {
	return nil
}
func (s *compactDialogueStore) ArchiveAndReplace(
	_ context.Context, params dialoguemanager.ArchiveAndReplaceParams,
) error {
	s.dialogue.Messages = params.Messages
	return nil
}
func (s *compactDialogueStore) List(context.Context) (
	iterator.Iterator[dialoguemanager.DialogueHeader], error,
) {
	return iterator.FromSlice([]dialoguemanager.DialogueHeader{s.dialogue.Header()}), nil
}

func TestCompactConversationModelSelection(t *testing.T) {
	chatModel := llmapi.ModelEntry{Provider: "openai", Name: "chat", ContextWindow: 100_000}
	compactModel := llmapi.ModelEntry{Provider: "anthropic", Name: "summary", ContextWindow: 100_000}

	for _, tc := range []struct {
		name      string
		args      []string
		wantModel llmapi.ModelEntry
	}{
		{"compact alias by default", []string{"compact", "rolling-fox"}, compactModel},
		{"explicit model", []string{"compact", "rolling-fox", "openai/chat"}, chatModel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := llmtest.New(
				[]llmapi.ModelEntry{chatModel, compactModel},
				llmtest.Response{Chunks: []string{"summary"}},
			)
			svc := &compactAliasService{Service: backend, target: compactModel}
			store := &compactDialogueStore{dialogue: dialoguemanager.Dialogue{
				ID: "rolling-fox",
				Messages: []llmapi.Message{
					{Role: llmapi.RoleSystem, Content: "system"},
					{Role: llmapi.RoleUser, Content: "question"},
				},
			}}
			s := &shell{
				llmSvc:       svc,
				defaultModel: "openai/chat",
				store:        store,
			}

			_, err := s.handleChats(t.Context(), tc.args)
			require.NoError(t, err)
			require.Len(t, backend.Requests(), 1)
			assert.Equal(t, tc.wantModel, backend.Requests()[0].Model)
		})
	}
}

func TestCompleteChatsCompactModel(t *testing.T) {
	s := &shell{llmSvc: llmtest.New([]llmapi.ModelEntry{
		{Provider: "openai", Name: "gpt-5"},
		{Provider: "codex", Name: "gpt-5"},
	})}

	it, err := s.Complete(t.Context(), "chats", []string{"compact", "rolling-fox", ""})
	require.NoError(t, err)
	got, err := iterator.ToSlice(t.Context(), it)
	require.NoError(t, err)
	assert.Equal(t, []string{"openai/gpt-5", "codex/gpt-5"}, got)
}
