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

package agentools

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"

	"unstable.build/rune/cmd/rune-agent/agent"
	"unstable.build/rune/cmd/rune-agent/configedit"
)

// memConfig is an in-memory configedit.Config implementation used to
// exercise the grep guard. It stores only force_builtin_tools because
// that is the only key the guard reads or writes.
type memConfig struct {
	mu             sync.Mutex
	hasForce       bool
	force          bool
	setCalls       []bool
	ephemeralCalls []bool
}

func newMemConfig() *memConfig { return &memConfig{} }

func withForce(v bool) *memConfig {
	m := newMemConfig()
	m.hasForce = true
	m.force = v
	return m
}

func (m *memConfig) GetBool(key string) configedit.Bool {
	return configedit.NewBool(func(context.Context) (bool, error) {
		m.mu.Lock()
		defer m.mu.Unlock()
		if key == forceBuiltinToolsKey && m.hasForce {
			return m.force, nil
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
	if key == forceBuiltinToolsKey {
		m.hasForce = true
		m.force = v
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

// ctxWithPrompter returns a context carrying p so bash/exec_command
// can pull it back out via agent.PrompterFromContext.
func ctxWithPrompter(p agent.Prompter) context.Context {
	return agent.WithPrompter(context.Background(), p)
}

func TestBash_force_builtin_tools(t *testing.T) {
	const grepCmd = `{"command": "grep foo .", "description": "search"}`

	makeExec := func() (workspaceapi.Executor, *atomic.Int32) {
		var n atomic.Int32
		rec := &recordingExec{startFn: func(_ context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
			n.Add(1)
			if cmd.Watcher != nil {
				cmd.Watcher.WatchProcess() <- nil
			}
			return 1, nil
		}}
		return rec, &n
	}

	t.Run("absent + yes runs command once, does not persist", func(t *testing.T) {
		ex, n := makeExec()
		mp := &mockPrompter{responses: []agent.PromptResponse{{Values: []string{"yes"}}}}
		cfg := newMemConfig()
		tool := newBash(ex, dirURI("/tmp"), cfg)
		res := tool.Execute(ctxWithPrompter(mp), grepCmd)
		assert.False(t, res.IsError, res.Content)
		assert.Equal(t, int32(1), n.Load())
		assert.Empty(t, cfg.setCalls)
	})

	t.Run("absent + no rejects without running, does not persist", func(t *testing.T) {
		ex, n := makeExec()
		mp := &mockPrompter{responses: []agent.PromptResponse{{Values: []string{"no"}}}}
		cfg := newMemConfig()
		tool := newBash(ex, dirURI("/tmp"), cfg)
		res := tool.Execute(ctxWithPrompter(mp), grepCmd)
		assert.True(t, res.IsError)
		assert.Contains(t, res.Content, "Do not use grep")
		assert.Contains(t, res.Content, "search_content")
		assert.Equal(t, int32(0), n.Load())
		assert.Empty(t, cfg.setCalls)
	})

	t.Run("absent + always runs and persists false (allow forever)", func(t *testing.T) {
		ex, n := makeExec()
		mp := &mockPrompter{responses: []agent.PromptResponse{{Values: []string{"always"}}}}
		cfg := newMemConfig()
		tool := newBash(ex, dirURI("/tmp"), cfg)
		res := tool.Execute(ctxWithPrompter(mp), grepCmd)
		assert.False(t, res.IsError, res.Content)
		assert.Equal(t, int32(1), n.Load())
		require.Len(t, cfg.setCalls, 1)
		assert.False(t, cfg.setCalls[0])
	})

	t.Run("absent + never rejects and persists true (reject forever)", func(t *testing.T) {
		ex, n := makeExec()
		mp := &mockPrompter{responses: []agent.PromptResponse{{Values: []string{"never"}}}}
		cfg := newMemConfig()
		tool := newBash(ex, dirURI("/tmp"), cfg)
		res := tool.Execute(ctxWithPrompter(mp), grepCmd)
		assert.True(t, res.IsError)
		assert.Equal(t, int32(0), n.Load())
		require.Len(t, cfg.setCalls, 1)
		assert.True(t, cfg.setCalls[0])
	})

	t.Run("absent + session_yes runs and stores ephemeral, no disk persist", func(t *testing.T) {
		ex, n := makeExec()
		mp := &mockPrompter{responses: []agent.PromptResponse{{Values: []string{"session_yes"}}}}
		cfg := newMemConfig()
		tool := newBash(ex, dirURI("/tmp"), cfg)
		res := tool.Execute(ctxWithPrompter(mp), grepCmd)
		assert.False(t, res.IsError, res.Content)
		assert.Equal(t, int32(1), n.Load())
		assert.Empty(t, cfg.setCalls)
		require.Len(t, cfg.ephemeralCalls, 1)
		assert.False(t, cfg.ephemeralCalls[0])
	})

	t.Run("absent + session_no rejects and stores ephemeral, no disk persist", func(t *testing.T) {
		ex, n := makeExec()
		mp := &mockPrompter{responses: []agent.PromptResponse{{Values: []string{"session_no"}}}}
		cfg := newMemConfig()
		tool := newBash(ex, dirURI("/tmp"), cfg)
		res := tool.Execute(ctxWithPrompter(mp), grepCmd)
		assert.True(t, res.IsError)
		assert.Equal(t, int32(0), n.Load())
		assert.Empty(t, cfg.setCalls)
		require.Len(t, cfg.ephemeralCalls, 1)
		assert.True(t, cfg.ephemeralCalls[0])
	})

	t.Run("session choice takes effect in same session (no second prompt)", func(t *testing.T) {
		ex, n := makeExec()
		mp := &mockPrompter{responses: []agent.PromptResponse{{Values: []string{"session_no"}}}}
		cfg := newMemConfig()
		tool := newBash(ex, dirURI("/tmp"), cfg)

		// First call: user picks "Not this session"; rejected, stored in
		// overlay only.
		res := tool.Execute(ctxWithPrompter(mp), grepCmd)
		assert.True(t, res.IsError)
		require.Len(t, mp.calls, 1)

		// Second call: must NOT prompt; overlay reads through.
		res = tool.Execute(ctxWithPrompter(mp), grepCmd)
		assert.True(t, res.IsError)
		assert.Equal(t, int32(0), n.Load())
		assert.Len(t, mp.calls, 1, "prompter must not be called a second time")
		assert.Empty(t, cfg.setCalls)
	})

	t.Run("persisted choice takes effect in same session (no second prompt)", func(t *testing.T) {
		ex, n := makeExec()
		mp := &mockPrompter{responses: []agent.PromptResponse{{Values: []string{"never"}}}}
		cfg := newMemConfig()
		tool := newBash(ex, dirURI("/tmp"), cfg)

		// First call: user picks "Never"; rejected and persisted.
		res := tool.Execute(ctxWithPrompter(mp), grepCmd)
		assert.True(t, res.IsError)
		require.Len(t, mp.calls, 1)

		// Second call: must NOT prompt, must reject because cfg now has
		// force_builtin_tools=true.
		res = tool.Execute(ctxWithPrompter(mp), grepCmd)
		assert.True(t, res.IsError)
		assert.Equal(t, int32(0), n.Load())
		assert.Len(t, mp.calls, 1, "prompter must not be called a second time")
	})

	t.Run("config true rejects without prompting", func(t *testing.T) {
		ex, n := makeExec()
		mp := &mockPrompter{}
		tool := newBash(ex, dirURI("/tmp"), withForce(true))
		res := tool.Execute(ctxWithPrompter(mp), grepCmd)
		assert.True(t, res.IsError)
		assert.Contains(t, res.Content, "Do not use grep")
		assert.Equal(t, int32(0), n.Load())
		assert.Empty(t, mp.calls)
	})

	t.Run("config false runs without prompting", func(t *testing.T) {
		ex, n := makeExec()
		mp := &mockPrompter{}
		tool := newBash(ex, dirURI("/tmp"), withForce(false))
		res := tool.Execute(ctxWithPrompter(mp), grepCmd)
		assert.False(t, res.IsError, res.Content)
		assert.Equal(t, int32(1), n.Load())
		assert.Empty(t, mp.calls)
	})

	t.Run("non-grep command bypasses guard entirely", func(t *testing.T) {
		ex, n := makeExec()
		mp := &mockPrompter{}
		tool := newBash(ex, dirURI("/tmp"), newMemConfig())
		res := tool.Execute(ctxWithPrompter(mp),
			`{"command": "go test ./...", "description": "tests"}`)
		assert.False(t, res.IsError, res.Content)
		assert.Equal(t, int32(1), n.Load())
		assert.Empty(t, mp.calls)
	})

	t.Run("nil prompter on grep command runs the command", func(t *testing.T) {
		ex, n := makeExec()
		tool := newBash(ex, dirURI("/tmp"), newMemConfig())
		res := tool.Execute(context.Background(), grepCmd)
		assert.False(t, res.IsError, res.Content)
		assert.Equal(t, int32(1), n.Load())
	})
}

func TestExecCommand_force_builtin_tools(t *testing.T) {
	const grepCmd = `{"cmd":"grep foo ."}`

	newTool := func(cfg configedit.Config) (agent.Tool, *SessionManager) {
		mgr := NewSessionManager(context.Background(), localExec{}, nil)
		return NewExecCommand(mgr, dirURI("/tmp"), cfg), mgr
	}

	t.Run("config true rejects with grep_files message", func(t *testing.T) {
		mp := &mockPrompter{}
		tool, _ := newTool(withForce(true))
		res := tool.Execute(ctxWithPrompter(mp), grepCmd)
		assert.True(t, res.IsError)
		assert.Contains(t, res.Content, "grep_files")
		assert.NotContains(t, res.Content, "search_content")
	})

	t.Run("absent + never rejects and persists true", func(t *testing.T) {
		mp := &mockPrompter{responses: []agent.PromptResponse{{Values: []string{"never"}}}}
		cfg := newMemConfig()
		tool, _ := newTool(cfg)
		res := tool.Execute(ctxWithPrompter(mp), grepCmd)
		assert.True(t, res.IsError)
		require.Len(t, cfg.setCalls, 1)
		assert.True(t, cfg.setCalls[0])
	})

	t.Run("config false runs without prompting", func(t *testing.T) {
		mp := &mockPrompter{}
		tool, _ := newTool(withForce(false))
		res := tool.Execute(ctxWithPrompter(mp), grepCmd)
		// May exit 0 or 1 depending on whether matches exist; the
		// important guarantee is that we did NOT hit the canonical
		// reject message.
		assert.NotContains(t, res.Content, "Do not use grep")
		assert.Empty(t, mp.calls)
	})
}
