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

package memory

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"unstable.build/rune/cmd/rune-agent/agent"
	"unstable.build/rune/cmd/rune-agent/configedit"
)

// memConfig is an in-memory configedit.Config used to exercise the
// GuardedRecaller. It only stores memory_recall_enabled because that is
// the only key the guard reads or writes.
type memConfig struct {
	mu             sync.Mutex
	hasEnabled     bool
	enabled        bool
	setCalls       []bool
	ephemeralCalls []bool
}

func newMemConfig() *memConfig { return &memConfig{} }

func withEnabled(v bool) *memConfig {
	m := newMemConfig()
	m.hasEnabled = true
	m.enabled = v
	return m
}

func (m *memConfig) GetBool(key string) configedit.Bool {
	return configedit.NewBool(func(context.Context) (bool, error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if key == memoryRecallEnabledKey && m.hasEnabled {
			return m.enabled, nil
		}
		return false, configedit.ErrNotFound
	})
}

func (m *memConfig) GetInt(string) configedit.Int             { return configedit.Int{} }
func (m *memConfig) GetFloat(string) configedit.Float         { return configedit.Float{} }
func (m *memConfig) GetString(string) configedit.String       { return configedit.String{} }
func (m *memConfig) GetMap(string) configedit.Map             { return configedit.Map{} }
func (m *memConfig) GetSlice(string) configedit.Slice         { return configedit.Slice{} }
func (m *memConfig) GetRune(string) configedit.Rune           { return configedit.Rune{} }
func (m *memConfig) GetColor(string) configedit.Color         { return configedit.Color{} }
func (m *memConfig) GetAttribute(string) configedit.Attribute { return configedit.Attribute{} }
func (m *memConfig) GetConfig(string) configedit.ConfigValue  { return configedit.ConfigValue{} }
func (m *memConfig) Iterate(func(string, any))                {}

func (m *memConfig) SetBool(_ context.Context, key string, v, ephemeral bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if key == memoryRecallEnabledKey {
		m.hasEnabled = true
		m.enabled = v
		if ephemeral {
			m.ephemeralCalls = append(m.ephemeralCalls, v)
		} else {
			m.setCalls = append(m.setCalls, v)
		}
	}
	return nil
}

func (m *memConfig) SetInt(context.Context, string, int, bool) error               { return nil }
func (m *memConfig) SetFloat(context.Context, string, float64, bool) error         { return nil }
func (m *memConfig) SetString(context.Context, string, string, bool) error         { return nil }
func (m *memConfig) AppendStringSlice(context.Context, string, string, bool) error { return nil }
func (m *memConfig) RemoveStringSlice(context.Context, string, string, bool) error { return nil }

// fakePrompter is a minimal agent.Prompter recorder used in
// GuardedRecaller tests.
type fakePrompter struct {
	response agent.PromptResponse
	err      error
	calls    []agent.PromptRequest
}

func (p *fakePrompter) Prompt(_ context.Context, req agent.PromptRequest) (agent.PromptResponse, error) {
	p.calls = append(p.calls, req)
	if p.err != nil {
		return agent.PromptResponse{}, p.err
	}
	return p.response, nil
}

// fakeRecaller is an agent.MemoryRecaller test double.
type fakeRecaller struct {
	memories []agent.Memory
	err      error
	calls    int
}

func (r *fakeRecaller) Recall(_ context.Context, _ []string, _ string, _ string) ([]agent.Memory, error) {
	r.calls++
	return r.memories, r.err
}

func TestNewGuardedRecaller_PanicsOnNil(t *testing.T) {
	t.Run("nil inner", func(t *testing.T) {
		assert.Panics(t, func() {
			NewGuardedRecaller(nil, newMemConfig())
		})
	})
	t.Run("nil cfg", func(t *testing.T) {
		assert.Panics(t, func() {
			NewGuardedRecaller(&fakeRecaller{}, nil)
		})
	})
}

func TestGuardedRecaller_Recall(t *testing.T) {
	someMems := []agent.Memory{
		{ID: "mem-1", Content: "Always test"},
	}

	t.Run("absent + inner returns 0 memories: no prompt, returns nil", func(t *testing.T) {
		inner := &fakeRecaller{memories: nil}
		cfg := newMemConfig()
		mp := &fakePrompter{}
		g := NewGuardedRecaller(inner, cfg)
		ctx := agent.WithPrompter(context.Background(), mp)

		got, err := g.Recall(ctx, nil, "task", "")
		require.NoError(t, err)
		assert.Nil(t, got)
		assert.Equal(t, 1, inner.calls)
		assert.Empty(t, mp.calls)
		assert.False(t, cfg.hasEnabled)
	})

	t.Run("absent + yes returns memories, does not persist", func(t *testing.T) {
		inner := &fakeRecaller{memories: someMems}
		cfg := newMemConfig()
		mp := &fakePrompter{response: agent.PromptResponse{Values: []string{"yes"}}}
		g := NewGuardedRecaller(inner, cfg)
		ctx := agent.WithPrompter(context.Background(), mp)

		got, err := g.Recall(ctx, nil, "task", "")
		require.NoError(t, err)
		assert.Equal(t, someMems, got)
		require.Len(t, mp.calls, 1)
		assert.Equal(t, "Allow memory recall?", mp.calls[0].Title)
		assert.Equal(t, "memory", mp.calls[0].Header)
		assert.False(t, cfg.hasEnabled)
	})

	t.Run("absent + always returns memories AND persists true", func(t *testing.T) {
		inner := &fakeRecaller{memories: someMems}
		cfg := newMemConfig()
		mp := &fakePrompter{response: agent.PromptResponse{Values: []string{"always"}}}
		g := NewGuardedRecaller(inner, cfg)
		ctx := agent.WithPrompter(context.Background(), mp)

		got, err := g.Recall(ctx, nil, "task", "")
		require.NoError(t, err)
		assert.Equal(t, someMems, got)
		require.Len(t, cfg.setCalls, 1)
		assert.True(t, cfg.setCalls[0])
		assert.True(t, cfg.enabled)
	})

	t.Run("absent + no returns nil, does not persist", func(t *testing.T) {
		inner := &fakeRecaller{memories: someMems}
		cfg := newMemConfig()
		mp := &fakePrompter{response: agent.PromptResponse{Values: []string{"no"}}}
		g := NewGuardedRecaller(inner, cfg)
		ctx := agent.WithPrompter(context.Background(), mp)

		got, err := g.Recall(ctx, nil, "task", "")
		require.NoError(t, err)
		assert.Nil(t, got)
		assert.False(t, cfg.hasEnabled)
	})

	t.Run("absent + never returns nil AND persists false", func(t *testing.T) {
		inner := &fakeRecaller{memories: someMems}
		cfg := newMemConfig()
		mp := &fakePrompter{response: agent.PromptResponse{Values: []string{"never"}}}
		g := NewGuardedRecaller(inner, cfg)
		ctx := agent.WithPrompter(context.Background(), mp)

		got, err := g.Recall(ctx, nil, "task", "")
		require.NoError(t, err)
		assert.Nil(t, got)
		require.Len(t, cfg.setCalls, 1)
		assert.False(t, cfg.setCalls[0])
		assert.True(t, cfg.hasEnabled)
		assert.False(t, cfg.enabled)
	})

	t.Run("absent + prompter dismissed returns nil, does not persist", func(t *testing.T) {
		inner := &fakeRecaller{memories: someMems}
		cfg := newMemConfig()
		mp := &fakePrompter{err: errors.New("dismissed")}
		g := NewGuardedRecaller(inner, cfg)
		ctx := agent.WithPrompter(context.Background(), mp)

		got, err := g.Recall(ctx, nil, "task", "")
		require.NoError(t, err)
		assert.Nil(t, got)
		assert.False(t, cfg.hasEnabled)
	})

	t.Run("absent + no prompter in ctx returns memories (default-allow)", func(t *testing.T) {
		inner := &fakeRecaller{memories: someMems}
		cfg := newMemConfig()
		g := NewGuardedRecaller(inner, cfg)

		got, err := g.Recall(context.Background(), nil, "task", "")
		require.NoError(t, err)
		assert.Equal(t, someMems, got)
		assert.False(t, cfg.hasEnabled)
	})

	t.Run("config true returns inner memories without prompting", func(t *testing.T) {
		inner := &fakeRecaller{memories: someMems}
		mp := &fakePrompter{}
		g := NewGuardedRecaller(inner, withEnabled(true))
		ctx := agent.WithPrompter(context.Background(), mp)

		got, err := g.Recall(ctx, nil, "task", "")
		require.NoError(t, err)
		assert.Equal(t, someMems, got)
		assert.Equal(t, 1, inner.calls)
		assert.Empty(t, mp.calls)
	})

	t.Run("config false returns nil without invoking inner", func(t *testing.T) {
		inner := &fakeRecaller{memories: someMems}
		mp := &fakePrompter{}
		g := NewGuardedRecaller(inner, withEnabled(false))
		ctx := agent.WithPrompter(context.Background(), mp)

		got, err := g.Recall(ctx, nil, "task", "")
		require.NoError(t, err)
		assert.Nil(t, got)
		assert.Equal(t, 0, inner.calls)
		assert.Empty(t, mp.calls)
	})

	t.Run("inner returns error: returned as-is, no prompt", func(t *testing.T) {
		wantErr := errors.New("boom")
		inner := &fakeRecaller{err: wantErr}
		cfg := newMemConfig()
		mp := &fakePrompter{}
		g := NewGuardedRecaller(inner, cfg)
		ctx := agent.WithPrompter(context.Background(), mp)

		_, err := g.Recall(ctx, nil, "task", "")
		require.ErrorIs(t, err, wantErr)
		assert.Empty(t, mp.calls)
		assert.False(t, cfg.hasEnabled)
	})

	t.Run("always choice is observable in same session (no re-prompt)", func(t *testing.T) {
		inner := &fakeRecaller{memories: someMems}
		cfg := newMemConfig()
		mp := &fakePrompter{response: agent.PromptResponse{Values: []string{"always"}}}
		g := NewGuardedRecaller(inner, cfg)
		ctx := agent.WithPrompter(context.Background(), mp)

		// First call: user picks "Always".
		got, err := g.Recall(ctx, nil, "task", "")
		require.NoError(t, err)
		assert.Equal(t, someMems, got)
		require.Len(t, mp.calls, 1)

		// Second call: must NOT prompt again; overlay reads through.
		got, err = g.Recall(ctx, nil, "task", "")
		require.NoError(t, err)
		assert.Equal(t, someMems, got)
		assert.Len(t, mp.calls, 1, "prompter must not be called a second time")
	})

	t.Run("never choice is observable in same session (no re-prompt)", func(t *testing.T) {
		inner := &fakeRecaller{memories: someMems}
		cfg := newMemConfig()
		mp := &fakePrompter{response: agent.PromptResponse{Values: []string{"never"}}}
		g := NewGuardedRecaller(inner, cfg)
		ctx := agent.WithPrompter(context.Background(), mp)

		// First call: user picks "Never"; rejected and persisted.
		got, err := g.Recall(ctx, nil, "task", "")
		require.NoError(t, err)
		assert.Nil(t, got)
		require.Len(t, mp.calls, 1)

		// Second call: must NOT prompt; inner must not be called again
		// because the overlay now reads false.
		callsBefore := inner.calls
		got, err = g.Recall(ctx, nil, "task", "")
		require.NoError(t, err)
		assert.Nil(t, got)
		assert.Len(t, mp.calls, 1, "prompter must not be called a second time")
		assert.Equal(t, callsBefore, inner.calls,
			"inner recaller must not be invoked once disabled")
	})
}
