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

package dialoguemanager

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
)

func TestGenerateUniqueID(t *testing.T) {
	ctx := context.Background()

	t.Run("generates ID with prefix", func(t *testing.T) {
		store := NewStore(storagestub.NewInMemoryService(), t.TempDir())
		id := GenerateUniqueID(ctx, store, "sub-agent-plan-")
		assert.True(t, strings.HasPrefix(id, "sub-agent-plan-"))
	})

	t.Run("generates ID without prefix", func(t *testing.T) {
		store := NewStore(storagestub.NewInMemoryService(), t.TempDir())
		id := GenerateUniqueID(ctx, store, "")
		assert.NotEmpty(t, id)
		// Petname format: word-word
		assert.Contains(t, id, "-")
	})

	t.Run("retries on collision", func(t *testing.T) {
		store := NewStore(storagestub.NewInMemoryService(), t.TempDir())

		// Generate first ID, then pre-create a dialogue with
		// the same ID to force a collision on next call.
		id1 := GenerateUniqueID(ctx, store, "test-")
		require.NoError(t, store.Create(ctx, Dialogue{ID: id1}))

		// Second call must produce a different ID.
		id2 := GenerateUniqueID(ctx, store, "test-")
		assert.NotEqual(t, id1, id2)
		assert.True(t, strings.HasPrefix(id2, "test-"))
	})

	t.Run("unique across multiple calls", func(t *testing.T) {
		store := NewStore(storagestub.NewInMemoryService(), t.TempDir())
		seen := make(map[string]bool)
		for range 20 {
			id := GenerateUniqueID(ctx, store, "")
			assert.False(t, seen[id], "duplicate ID: %s", id)
			seen[id] = true
			// Persist each ID so the store check can find it.
			require.NoError(t, store.Create(ctx, Dialogue{ID: id}))
		}
	})
}
