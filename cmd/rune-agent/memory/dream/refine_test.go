// Copyright (C) 2017-2026 Unstable Build, LLC
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

package dream

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/rune/cmd/rune-agent/dialogue/dialoguemanager"
)

func TestDialogueHasMemoryContext(t *testing.T) {
	t.Run("returns true when user message contains memory-context block", func(t *testing.T) {
		d := dialoguemanager.Dialogue{
			Messages: []llmapi.Message{
				{Role: llmapi.RoleUser, Content: "<memory-context>\n[m1] foo\n</memory-context>\n\nhello"},
			},
		}
		assert.True(t, dialogueHasMemoryContext(d))
	})

	t.Run("returns false when user message has no block", func(t *testing.T) {
		d := dialoguemanager.Dialogue{
			Messages: []llmapi.Message{
				{Role: llmapi.RoleUser, Content: "plain question"},
				{Role: llmapi.RoleAssistant, Content: "<memory-context>not in user msg</memory-context>"},
			},
		}
		assert.False(t, dialogueHasMemoryContext(d))
	})

	t.Run("returns false when only opening tag is present", func(t *testing.T) {
		d := dialoguemanager.Dialogue{
			Messages: []llmapi.Message{
				{Role: llmapi.RoleUser, Content: "<memory-context> unclosed"},
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
				{ID: "d1", Version: 1, Messages: []llmapi.Message{
					{Role: llmapi.RoleUser, Content: "no recall block"},
				}},
			},
		}
		storage := deps.Storage.(*mockStorage)
		storage.data[dreamStateID] = &DreamState{
			SchemaVersion: templateVersion,
			Dreamed:       map[string]int64{"d1": 1},
		}

		prompt, err := buildRefineUserPrompt(context.Background(), deps)
		require.NoError(t, err)
		assert.Empty(t, prompt)
	})

	t.Run("returns prompt with transcripts when memory-context present", func(t *testing.T) {
		deps := validDeps(t, t.TempDir())
		deps.Store = &mockDialogueStore{
			dialogues: []dialoguemanager.Dialogue{
				{ID: "d1", Version: 1, Messages: []llmapi.Message{
					{Role: llmapi.RoleUser, Content: "<memory-context>\n[m1] foo\n</memory-context>\n\nhelp"},
					{Role: llmapi.RoleAssistant, Content: "sure"},
				}},
				{ID: "d2", Version: 1, Messages: []llmapi.Message{
					{Role: llmapi.RoleUser, Content: "no block here"},
				}},
			},
		}
		storage := deps.Storage.(*mockStorage)
		storage.data[dreamStateID] = &DreamState{
			SchemaVersion: templateVersion,
			Dreamed:       map[string]int64{"d1": 1, "d2": 1},
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
			Dreamed:       map[string]int64{"d-missing": 1},
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
