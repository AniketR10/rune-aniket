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

package llmrouter

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
)

func me(provider, name string) llmapi.ModelEntry {
	return llmapi.ModelEntry{Provider: provider, Name: name}
}

func TestAliasStore_SetGet(t *testing.T) {
	ctx := context.Background()
	s := newAliasStore(storagestub.NewInMemoryService())

	require.NoError(t, s.set(ctx, "default", me("openai", "gpt-5.5")))
	target, ok, err := s.get(ctx, "default")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, me("openai", "gpt-5.5"), target)

	_, ok, err = s.get(ctx, "missing")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestAliasStore_SetOverwrites(t *testing.T) {
	ctx := context.Background()
	s := newAliasStore(storagestub.NewInMemoryService())

	require.NoError(t, s.set(ctx, "default", me("openai", "gpt-5.5")))
	require.NoError(t, s.set(ctx, "default", me("anthropic", "claude-opus-4-8")))
	target, ok, err := s.get(ctx, "default")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, me("anthropic", "claude-opus-4-8"), target)
}

func TestAliasStore_Remove(t *testing.T) {
	ctx := context.Background()
	s := newAliasStore(storagestub.NewInMemoryService())
	require.NoError(t, s.set(ctx, "default", me("openai", "gpt-5.5")))

	require.NoError(t, s.remove(ctx, "default"))
	_, ok, err := s.get(ctx, "default")
	require.NoError(t, err)
	assert.False(t, ok)

	err = s.remove(ctx, "default")
	assert.ErrorIs(t, err, ErrAliasNotFound)
}

func TestAliasStore_All(t *testing.T) {
	ctx := context.Background()
	s := newAliasStore(storagestub.NewInMemoryService())
	require.NoError(t, s.set(ctx, "default", me("openai", "gpt-5.5")))
	require.NoError(t, s.set(ctx, "query", me("gemini", "gemini-3.1-pro-preview")))

	all, err := s.all(ctx)
	require.NoError(t, err)
	assert.Equal(t, map[string]llmapi.ModelEntry{
		"default": me("openai", "gpt-5.5"),
		"query":   me("gemini", "gemini-3.1-pro-preview"),
	}, all)
}

func TestAliasStore_EmptyNameOrTarget(t *testing.T) {
	ctx := context.Background()
	s := newAliasStore(storagestub.NewInMemoryService())
	assert.Error(t, s.set(ctx, "", me("openai", "gpt-5.5")))
	assert.Error(t, s.set(ctx, "default", llmapi.ModelEntry{}))
}

func TestAliasStore_NilStoragePanics(t *testing.T) {
	assert.PanicsWithValue(t,
		"llmrouter: newAliasStore: storage must not be nil",
		func() { newAliasStore(nil) })
}

func TestAliasStore_ConcurrentSetsNoLostWrites(t *testing.T) {
	ctx := context.Background()
	s := newAliasStore(storagestub.NewInMemoryService())

	const n = 16
	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		go func(i int) {
			defer wg.Done()
			assert.NoError(t, s.set(ctx, fmt.Sprintf("a%02d", i), me("openai", fmt.Sprintf("m%02d", i))))
		}(i)
	}
	wg.Wait()

	all, err := s.all(ctx)
	require.NoError(t, err)
	assert.Len(t, all, n)
}
