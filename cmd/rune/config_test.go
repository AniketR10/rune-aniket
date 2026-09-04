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

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/term"
	"go.uber.org/mock/gomock"

	"unstable.build/rune/browser/browsertest"
	"unstable.build/rune/ide"
)

// TestIDEConfigOverlaySubscriptResolvesDefaultTree is a regression test for
// ide.Config decoding user configs against a bare `config = {}` default
// instead of the full rune.star tree. An overlay-style user config that
// mutates a nested default key (config["terminal"]["initial_reservoir"] = 2)
// used to fail with `key "terminal" not in dict` on the gui.env load paths.
// Requiring the DefaultConfig argument keeps every ide.Config caller on the
// same baseline the running IDE uses.
func TestIDEConfigOverlaySubscriptResolvesDefaultTree(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.star")
	require.NoError(t, os.WriteFile(path,
		[]byte("config[\"terminal\"][\"initial_reservoir\"] = 2\n"), 0o644))

	cfg, err := ide.Config(path, runeDefaultConfig())
	require.NoError(t, err)

	term, err := cfg.GetConfig("terminal")
	require.NoError(t, err)
	got, err := term.GetInt("initial_reservoir")
	require.NoError(t, err)
	assert.Equal(t, 2, got)
}

// TestDefaultQuickMenuButtons guards the quick menu shipped in rune.star
// against an entry that validation rejects, which would silently drop a
// button at runtime. It compares parsed buttons against raw entries
// rather than pinning the list, so editing the menu does not fail here.
func TestDefaultQuickMenuButtons(t *testing.T) {
	cfg, err := ide.Config(filepath.Join(t.TempDir(), "config.star"),
		runeDefaultConfig())
	require.NoError(t, err)

	guiCfg, err := cfg.GetConfig("gui")
	require.NoError(t, err)
	entries, err := guiCfg.GetSlice("quick_menu")
	require.NoError(t, err)
	require.NotEmpty(t, entries)

	buttons := ide.QuickMenuButtons(cfg)
	assert.Len(t, buttons, len(entries),
		"every shipped quick menu entry must survive validation")

	seen := make(map[string]bool, len(buttons))
	for _, button := range buttons {
		assert.NotEmpty(t, button.Symbol, "%q has no symbol", button.ID())
		assert.NotEmpty(t, button.Title, "%q has no title", button.ID())
		assert.False(t, seen[button.ID()], "duplicate command %q", button.ID())
		seen[button.ID()] = true
	}
}

func TestGetGUIKeyMapping(t *testing.T) {
	t.Run("parses valid mappings", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		b := browsertest.NewMockBrowser(ctrl)

		cfg := config.MapConfig(map[string]any{
			"key_mapping": map[string]any{
				"<capslock>": "<esc>",
				"<numlock>":  "a",
			},
		})

		got := getGUIKeyMapping(b, cfg)
		want := map[term.KeyComb]term.KeyComb{
			{Key: term.KeyCapsLock}: {Key: term.KeyEsc},
			{Key: term.KeyNumLock}:  {Ch: 'a'},
		}
		assert.Equal(t, want, got)
	})

	t.Run("returns nil when absent", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		b := browsertest.NewMockBrowser(ctrl)

		got := getGUIKeyMapping(b, config.MapConfig(map[string]any{}))
		assert.Nil(t, got)
	})

	t.Run("skips invalid entries and notifies", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		b := browsertest.NewMockBrowser(ctrl)
		// One report for the bad source, one for the bad target, one for
		// the non-string value.
		b.EXPECT().Notify(gomock.Any(), gomock.Any(), gomock.Any()).
			Return("", nil).Times(3)

		cfg := config.MapConfig(map[string]any{
			"key_mapping": map[string]any{
				"<capslock>":   "<esc>",
				"<not-a-key>":  "<esc>",
				"<numlock>":    "<also-bad>",
				"<scrolllock>": 42,
			},
		})

		got := getGUIKeyMapping(b, cfg)
		want := map[term.KeyComb]term.KeyComb{
			{Key: term.KeyCapsLock}: {Key: term.KeyEsc},
		}
		assert.Equal(t, want, got)
	})
}

func TestGetGUIFontSize(t *testing.T) {
	t.Run("returns 0 when absent to select DPI-aware default", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		b := browsertest.NewMockBrowser(ctrl)

		got := getGUIFontSize(b, config.MapConfig(map[string]any{}))
		assert.Equal(t, float64(0), got)
	})

	t.Run("returns configured size when set", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		b := browsertest.NewMockBrowser(ctrl)

		cfg := config.MapConfig(map[string]any{"font_size": 15.0})
		got := getGUIFontSize(b, cfg)
		assert.Equal(t, float64(15), got)
	})
}
