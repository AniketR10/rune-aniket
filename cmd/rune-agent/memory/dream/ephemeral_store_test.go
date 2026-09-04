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
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"unstable.build/rune/cmd/rune-agent/dialogue/dialoguemanager"
)

func TestEphemeralStore(t *testing.T) {
	ctx := context.Background()

	t.Run("Health returns nil", func(t *testing.T) {
		s := newEphemeralStore()
		assert.NoError(t, s.Health(ctx))
	})

	t.Run("Create and Get", func(t *testing.T) {
		s := newEphemeralStore()
		msgs := []llmapi.Message{
			{Role: llmapi.RoleUser, Content: "hello"},
		}
		require.NoError(t, s.Create(ctx, dialoguemanager.Dialogue{
			ID: "d1", Messages: msgs,
		}))

		d, err := s.Get(ctx, "d1")
		require.NoError(t, err)
		assert.Equal(t, "d1", d.ID)
		assert.Len(t, d.Messages, 1)
		assert.Equal(t, "hello", d.Messages[0].Content)
		assert.Equal(t, int64(1), d.Version)
	})

	t.Run("Create returns ErrAlreadyExists", func(t *testing.T) {
		s := newEphemeralStore()
		require.NoError(t, s.Create(ctx, dialoguemanager.Dialogue{ID: "d1"}))

		err := s.Create(ctx, dialoguemanager.Dialogue{ID: "d1"})
		assert.ErrorIs(t, err, storageapi.ErrAlreadyExists)
	})

	t.Run("Get returns ErrNotFound", func(t *testing.T) {
		s := newEphemeralStore()
		_, err := s.Get(ctx, "nonexistent")
		assert.ErrorIs(t, err, storageapi.ErrNotFound)
	})

	t.Run("Delete removes dialogue", func(t *testing.T) {
		s := newEphemeralStore()
		require.NoError(t, s.Create(ctx, dialoguemanager.Dialogue{ID: "d1"}))
		require.NoError(t, s.Delete(ctx, "d1"))

		_, err := s.Get(ctx, "d1")
		assert.ErrorIs(t, err, storageapi.ErrNotFound)
	})

	t.Run("AppendMessages appends and increments version", func(t *testing.T) {
		s := newEphemeralStore()
		require.NoError(t, s.Create(ctx, dialoguemanager.Dialogue{
			ID: "d1", Messages: []llmapi.Message{
				{Role: llmapi.RoleUser, Content: "first"},
			},
		}))

		d, err := s.Get(ctx, "d1")
		require.NoError(t, err)

		err = s.AppendMessages(ctx, d, []llmapi.Message{
			{Role: llmapi.RoleAssistant, Content: "second"},
		}, llmapi.DialogueUsage{})
		require.NoError(t, err)

		d, err = s.Get(ctx, "d1")
		require.NoError(t, err)
		assert.Len(t, d.Messages, 2)
		assert.Equal(t, "second", d.Messages[1].Content)
		assert.Equal(t, int64(2), d.Version)
	})

	t.Run("AppendMessages returns ErrNotFound for missing dialogue", func(t *testing.T) {
		s := newEphemeralStore()
		err := s.AppendMessages(ctx, dialoguemanager.Dialogue{ID: "nope"}, nil, llmapi.DialogueUsage{})
		assert.ErrorIs(t, err, storageapi.ErrNotFound)
	})

	t.Run("List returns all dialogues", func(t *testing.T) {
		s := newEphemeralStore()
		require.NoError(t, s.Create(ctx, dialoguemanager.Dialogue{ID: "d1"}))
		require.NoError(t, s.Create(ctx, dialoguemanager.Dialogue{ID: "d2"}))

		it, err := s.List(ctx)
		require.NoError(t, err)
		defer func() { _ = it.Close() }()

		var ids []string
		for {
			d, ok := it.Next(ctx)
			if !ok {
				break
			}
			ids = append(ids, d.ID)
		}
		assert.NoError(t, it.Err())
		assert.Len(t, ids, 2)
		assert.Contains(t, ids, "d1")
		assert.Contains(t, ids, "d2")
	})
}
