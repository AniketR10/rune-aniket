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

package dream

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
	"unstable.build/go-tui/cmd/rune-agent/llm"
)

func TestDialogueHasMemoryContext(t *testing.T) {
	t.Run("returns true when user message contains memory-context block", func(t *testing.T) {
		d := dialoguemanager.Dialogue{
			Messages: []llm.Message{
				{Role: llm.RoleUser, Content: "<memory-context>\n[m1] foo\n</memory-context>\n\nhello"},
			},
		}
		assert.True(t, dialogueHasMemoryContext(d))
	})

	t.Run("returns false when user message has no block", func(t *testing.T) {
		d := dialoguemanager.Dialogue{
			Messages: []llm.Message{
				{Role: llm.RoleUser, Content: "plain question"},
				{Role: llm.RoleAssistant, Content: "<memory-context>not in user msg</memory-context>"},
			},
		}
		assert.False(t, dialogueHasMemoryContext(d))
	})

	t.Run("returns false when only opening tag is present", func(t *testing.T) {
		d := dialoguemanager.Dialogue{
			Messages: []llm.Message{
				{Role: llm.RoleUser, Content: "<memory-context> unclosed"},
			},
		}
		assert.False(t, dialogueHasMemoryContext(d))
	})
}

func TestBuildRefineUserPrompt(t *testing.T) {
	t.Run("returns empty when no dialogues carry memory-context", func(t *testing.T) {
		deps := validDeps(t, t.TempDir())
		deps.Store = &mockDialogueStore{
			dialogues: []dialoguemanager.Dialogue{
				{ID: "d1", Version: 1, Messages: []llm.Message{
					{Role: llm.RoleUser, Content: "no recall block"},
				}},
			},
		}
		storage := deps.Storage.(*mockStorage)
		storage.data[dreamStateID] = &DreamState{
			SchemaVersion: templateVersion,
			Dreamed:       map[string]int{"d1": 1},
		}

		prompt, err := buildRefineUserPrompt(context.Background(), deps)
		require.NoError(t, err)
		assert.Empty(t, prompt)
	})

	t.Run("returns prompt with transcripts when memory-context present", func(t *testing.T) {
		deps := validDeps(t, t.TempDir())
		deps.Store = &mockDialogueStore{
			dialogues: []dialoguemanager.Dialogue{
				{ID: "d1", Version: 1, Messages: []llm.Message{
					{Role: llm.RoleUser, Content: "<memory-context>\n[m1] foo\n</memory-context>\n\nhelp"},
					{Role: llm.RoleAssistant, Content: "sure"},
				}},
				{ID: "d2", Version: 1, Messages: []llm.Message{
					{Role: llm.RoleUser, Content: "no block here"},
				}},
			},
		}
		storage := deps.Storage.(*mockStorage)
		storage.data[dreamStateID] = &DreamState{
			SchemaVersion: templateVersion,
			Dreamed:       map[string]int{"d1": 1, "d2": 1},
		}

		prompt, err := buildRefineUserPrompt(context.Background(), deps)
		require.NoError(t, err)
		assert.Contains(t, prompt, "Dialogue d1")
		assert.NotContains(t, prompt, "Dialogue d2")
		assert.Contains(t, prompt, "<memory-context>")
	})

	t.Run("skips dialogues that no longer exist in the store", func(t *testing.T) {
		deps := validDeps(t, t.TempDir())
		deps.Store = &mockDialogueStore{} // Get returns ErrNotFound
		storage := deps.Storage.(*mockStorage)
		storage.data[dreamStateID] = &DreamState{
			SchemaVersion: templateVersion,
			Dreamed:       map[string]int{"d-missing": 1},
		}

		prompt, err := buildRefineUserPrompt(context.Background(), deps)
		require.NoError(t, err)
		assert.Empty(t, prompt)
	})
}

func TestRefinePhaseRegistered(t *testing.T) {
	var found bool
	for _, p := range phases {
		if p.Name == "refine" {
			found = true
			break
		}
	}
	assert.True(t, found, "refine phase should be registered in phases")
}
