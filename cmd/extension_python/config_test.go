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

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
)

func TestApplyPyConfig(t *testing.T) {
	defCommand := "ty server"
	defAlternates := map[string]string{
		"textDocument/formatting":      "ruff server",
		"textDocument/rangeFormatting": "ruff server",
	}

	t.Run("nil config keeps defaults", func(t *testing.T) {
		cmd, alt := applyPyConfig(nil, newFakeNotifications(), defCommand, defAlternates)
		assert.Equal(t, defCommand, cmd)
		assert.Equal(t, defAlternates, alt)
	})

	t.Run("empty config keeps defaults", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{})
		cmd, alt := applyPyConfig(cfg, newFakeNotifications(), defCommand, defAlternates)
		assert.Equal(t, defCommand, cmd)
		assert.Equal(t, defAlternates, alt)
	})

	t.Run("command override drops default alternates", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{
			"command": "pyright-langserver --stdio",
		})
		cmd, alt := applyPyConfig(cfg, newFakeNotifications(), defCommand, defAlternates)
		assert.Equal(t, "pyright-langserver --stdio", cmd)
		assert.Nil(t, alt)
	})

	t.Run("command override keeps supplied alternates", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{
			"command": "pyright-langserver --stdio",
			"alternate_commands": map[string]any{
				"textDocument/formatting": "ruff server",
			},
		})
		cmd, alt := applyPyConfig(cfg, newFakeNotifications(), defCommand, defAlternates)
		assert.Equal(t, "pyright-langserver --stdio", cmd)
		assert.Equal(t, map[string]string{"textDocument/formatting": "ruff server"}, alt)
	})

	t.Run("alternates override without command keeps default command", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{
			"alternate_commands": map[string]any{
				"textDocument/formatting": "black server",
			},
		})
		cmd, alt := applyPyConfig(cfg, newFakeNotifications(), defCommand, defAlternates)
		assert.Equal(t, defCommand, cmd)
		assert.Equal(t, map[string]string{"textDocument/formatting": "black server"}, alt)
	})

	t.Run("invalid alternate value warns and keeps defaults", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{
			"alternate_commands": map[string]any{
				"textDocument/formatting": 42,
			},
		})
		notify := newFakeNotifications()
		cmd, alt := applyPyConfig(cfg, notify, defCommand, defAlternates)
		assert.Equal(t, defCommand, cmd)
		assert.Equal(t, defAlternates, alt)
		assert.NotEmpty(t, notify.notifs)
	})
}

func TestPyLogLevel(t *testing.T) {
	t.Run("defaults to server default", func(t *testing.T) {
		assert.Empty(t, pyLogLevel(nil, newFakeNotifications()))
	})

	t.Run("reads debug level", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{
			"debug": map[string]any{"log_level": "debug"},
		})
		assert.Equal(t, "debug", pyLogLevel(cfg, newFakeNotifications()))
	})

	t.Run("rejects invalid level", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{
			"debug": map[string]any{"log_level": "verbose"},
		})
		notify := newFakeNotifications()
		assert.Empty(t, pyLogLevel(cfg, notify))
		assert.NotEmpty(t, notify.notifs)
	})

	t.Run("invalid level without notifications", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{
			"debug": map[string]any{"log_level": "verbose"},
		})
		assert.Empty(t, pyLogLevel(cfg, nil))
	})
}

func TestPyWatchEvents(t *testing.T) {
	onChange := []textapi.EventType{
		textapi.EventTypeOpen, textapi.EventTypeChange, textapi.EventTypeCreate,
	}
	openOnly := []textapi.EventType{textapi.EventTypeOpen}

	t.Run("absent key enables change/create", func(t *testing.T) {
		assert.Equal(t, onChange, pyWatchEvents(config.NopConfig(), newFakeNotifications()))
	})

	t.Run("true enables change/create", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{"watch_events": true})
		assert.Equal(t, onChange, pyWatchEvents(cfg, newFakeNotifications()))
	})

	t.Run("false restores open-only", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{"watch_events": false})
		assert.Equal(t, openOnly, pyWatchEvents(cfg, newFakeNotifications()))
	})

	t.Run("non-bool warns and defaults to change/create", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{"watch_events": "yes"})
		notify := newFakeNotifications()
		assert.Equal(t, onChange, pyWatchEvents(cfg, notify))
		assert.NotEmpty(t, notify.notifs)
	})
}

func TestPyDiagnosticMode(t *testing.T) {
	t.Run("nil config returns empty", func(t *testing.T) {
		assert.Equal(t, "", pyDiagnosticMode(nil, newFakeNotifications()))
	})

	t.Run("absent key returns empty", func(t *testing.T) {
		assert.Equal(t, "", pyDiagnosticMode(config.NopConfig(), newFakeNotifications()))
	})

	t.Run("workspace accepted", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{"diagnostic_mode": "workspace"})
		notify := newFakeNotifications()
		assert.Equal(t, "workspace", pyDiagnosticMode(cfg, notify))
		assert.Empty(t, notify.notifs)
	})

	t.Run("openFilesOnly accepted", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{"diagnostic_mode": "openFilesOnly"})
		assert.Equal(t, "openFilesOnly", pyDiagnosticMode(cfg, newFakeNotifications()))
	})

	t.Run("off accepted", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{"diagnostic_mode": "off"})
		assert.Equal(t, "off", pyDiagnosticMode(cfg, newFakeNotifications()))
	})

	t.Run("unknown value warns and is ignored", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{"diagnostic_mode": "all"})
		notify := newFakeNotifications()
		assert.Equal(t, "", pyDiagnosticMode(cfg, notify))
		assert.NotEmpty(t, notify.notifs)
	})

	t.Run("non-string warns and is ignored", func(t *testing.T) {
		cfg := config.JSONFromMap(map[string]any{"diagnostic_mode": 3})
		notify := newFakeNotifications()
		assert.Equal(t, "", pyDiagnosticMode(cfg, notify))
		assert.NotEmpty(t, notify.notifs)
	})
}
