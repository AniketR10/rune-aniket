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

package configedit_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/cmd/rune-agent/configedit"

	"github.com/unstablebuild/rune-go-sdk/term"
)

type osFS struct{}

func (osFS) OpenFile(p string, flag int, perm os.FileMode) (workspaceapi.File, error) {
	return os.OpenFile(p, flag, perm)
}

func (osFS) MkdirAll(p string, perm os.FileMode) error { return os.MkdirAll(p, perm) }

func dirURI(t *testing.T) (string, workspaceapi.URI) {
	t.Helper()
	root := t.TempDir()
	u, err := workspaceapi.ParseURI("file://" + root)
	require.NoError(t, err)
	return root, u
}

func readConfigFile(t *testing.T, root string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, ".rune", "config.yaml"))
	require.NoError(t, err)
	return string(data)
}

func TestNewConfig_panics_on_nil_fs(t *testing.T) {
	assert.Panics(t, func() {
		_ = configedit.NewConfig(nil, workspaceapi.URI{}, nil)
	})
}

func TestEditor_overlay_takes_precedence_over_snapshot(t *testing.T) {
	_, cwd := dirURI(t)
	snapshot := config.JSONFromMap(map[string]any{
		"force_builtin_tools": true,
		"max_tokens":          1000,
	})
	cfg := configedit.NewConfig(osFS{}, cwd, snapshot)

	ctx := context.Background()

	// Before overrides, snapshot values are returned.
	v, err := cfg.GetBool("force_builtin_tools").Resolve(ctx)
	require.NoError(t, err)
	assert.True(t, v)

	n, err := cfg.GetInt("max_tokens").Resolve(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1000, n)

	// Override via Setter — Resolve must now return the new value.
	require.NoError(t, cfg.SetBool(ctx, "force_builtin_tools", false, false))
	require.NoError(t, cfg.SetInt(ctx, "max_tokens", 2000, false))

	v, err = cfg.GetBool("force_builtin_tools").Resolve(ctx)
	require.NoError(t, err)
	assert.False(t, v, "overlay should win over snapshot")

	n, err = cfg.GetInt("max_tokens").Resolve(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2000, n, "overlay should win over snapshot")
}

func TestEditor_persists_writes_to_disk(t *testing.T) {
	root, cwd := dirURI(t)
	cfg := configedit.NewConfig(osFS{}, cwd, nil)
	ctx := context.Background()

	require.NoError(t, cfg.SetBool(ctx, "force_builtin_tools", true, false))
	require.NoError(t, cfg.SetInt(ctx, "max_tokens", 4096, false))
	require.NoError(t, cfg.SetString(ctx, "agents_file", "AGENTS.md", false))

	content := readConfigFile(t, root)
	assert.Contains(t, content, "force_builtin_tools: true")
	assert.Contains(t, content, "max_tokens: 4096")
	assert.Contains(t, content, "agents_file: AGENTS.md")
	assert.Contains(t, content, "extensions")
	assert.Contains(t, content, "rune-agent")
}

func TestEditor_ephemeral_does_not_persist_to_disk(t *testing.T) {
	root, cwd := dirURI(t)
	cfg := configedit.NewConfig(osFS{}, cwd, nil)
	ctx := context.Background()

	require.NoError(t, cfg.SetBool(ctx, "force_builtin_tools", true, true))
	require.NoError(t, cfg.SetInt(ctx, "max_tokens", 4096, true))
	require.NoError(t, cfg.SetString(ctx, "agents_file", "AGENTS.md", true))
	require.NoError(t, cfg.AppendStringSlice(ctx, "skills", "a/path", true))

	// Subsequent reads observe the overlay values.
	v, err := cfg.GetBool("force_builtin_tools").Resolve(ctx)
	require.NoError(t, err)
	assert.True(t, v)
	n, err := cfg.GetInt("max_tokens").Resolve(ctx)
	require.NoError(t, err)
	assert.Equal(t, 4096, n)
	s, err := cfg.GetString("agents_file").Resolve(ctx)
	require.NoError(t, err)
	assert.Equal(t, "AGENTS.md", s)
	sl, err := cfg.GetSlice("skills").Resolve(ctx)
	require.NoError(t, err)
	require.Len(t, sl, 1)
	assert.Equal(t, "a/path", sl[0])

	// But .rune/config.yaml must not have been created.
	_, err = os.Stat(filepath.Join(root, ".rune", "config.yaml"))
	assert.True(t, os.IsNotExist(err), "ephemeral writes must not create the config file")
}

func TestEditor_ephemeral_does_not_overwrite_existing_file(t *testing.T) {
	root, cwd := dirURI(t)
	cfg := configedit.NewConfig(osFS{}, cwd, nil)
	ctx := context.Background()

	// Persist one value so the file exists with known content.
	require.NoError(t, cfg.SetBool(ctx, "force_builtin_tools", false, false))
	before := readConfigFile(t, root)

	// Ephemeral overrides must not modify the file.
	require.NoError(t, cfg.SetBool(ctx, "force_builtin_tools", true, true))
	require.NoError(t, cfg.AppendStringSlice(ctx, "skills", "a/path", true))
	require.NoError(t, cfg.RemoveStringSlice(ctx, "skills", "a/path", true))

	after := readConfigFile(t, root)
	assert.Equal(t, before, after, "ephemeral writes must not touch the file")

	// And the overlay still reflects the ephemeral value.
	v, err := cfg.GetBool("force_builtin_tools").Resolve(ctx)
	require.NoError(t, err)
	assert.True(t, v)
}

func TestEditor_missing_key_returns_ErrNotFound(t *testing.T) {
	_, cwd := dirURI(t)
	cfg := configedit.NewConfig(osFS{}, cwd, nil)
	_, err := cfg.GetString("missing").Resolve(context.Background())
	assert.ErrorIs(t, err, configedit.ErrNotFound)
}

func TestEditor_AppendStringSlice_and_RemoveStringSlice(t *testing.T) {
	root, cwd := dirURI(t)
	cfg := configedit.NewConfig(osFS{}, cwd, nil)
	ctx := context.Background()

	require.NoError(t, cfg.AppendStringSlice(ctx, "skills", "a/path", false))
	require.NoError(t, cfg.AppendStringSlice(ctx, "skills", "b/path", false))

	got, err := cfg.GetSlice("skills").Resolve(ctx)
	require.NoError(t, err)
	require.Len(t, got, 2)

	dup := cfg.AppendStringSlice(ctx, "skills", "a/path", false)
	assert.ErrorIs(t, dup, configedit.ErrAlreadyPresent)

	require.NoError(t, cfg.RemoveStringSlice(ctx, "skills", "a/path", false))

	got, err = cfg.GetSlice("skills").Resolve(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "b/path", got[0])

	miss := cfg.RemoveStringSlice(ctx, "skills", "missing", false)
	assert.ErrorIs(t, miss, configedit.ErrNotPresent)

	content := readConfigFile(t, root)
	assert.Contains(t, content, "b/path")
	assert.NotContains(t, content, "a/path")
}

func TestEditor_nested_GetConfig_reads_snapshot(t *testing.T) {
	_, cwd := dirURI(t)
	snapshot := config.JSONFromMap(map[string]any{
		"openai": map[string]any{
			"api_key": "sk-test",
		},
	})
	cfg := configedit.NewConfig(osFS{}, cwd, snapshot)

	sub, err := cfg.GetConfig("openai").Resolve(context.Background())
	require.NoError(t, err)
	key, err := sub.GetString("api_key").Resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "sk-test", key)
}

func TestNopConfig(t *testing.T) {
	cfg := configedit.NopConfig()
	ctx := context.Background()
	_, err := cfg.GetBool("x").Resolve(ctx)
	assert.ErrorIs(t, err, configedit.ErrNotFound)
	assert.NoError(t, cfg.SetBool(ctx, "x", true, false))
	assert.NoError(t, cfg.AppendStringSlice(ctx, "y", "z", false))
}

func TestEditor_Resolve_zero_value_returns_ErrNotFound(t *testing.T) {
	var b configedit.Bool
	_, err := b.Resolve(context.Background())
	assert.ErrorIs(t, err, configedit.ErrNotFound)
}

func TestFromSnapshot_panics_on_nil_snapshot(t *testing.T) {
	assert.PanicsWithValue(t,
		"configedit: FromSnapshot called with nil snapshot; use NopConfig() instead",
		func() { configedit.FromSnapshot(nil) },
	)
}

func TestEditor_GetMap_overlay_and_snapshot(t *testing.T) {
	_, cwd := dirURI(t)
	snapshot := config.JSONFromMap(map[string]any{
		"hooks": map[string]any{"on_save": "echo hi"},
	})
	cfg := configedit.NewConfig(osFS{}, cwd, snapshot)
	ctx := context.Background()

	// Snapshot read-through.
	m, err := cfg.GetMap("hooks").Resolve(ctx)
	require.NoError(t, err)
	assert.Equal(t, "echo hi", m["on_save"])

	// Missing key.
	_, err = cfg.GetMap("missing").Resolve(ctx)
	assert.ErrorIs(t, err, configedit.ErrNotFound)

	// Wrong type in snapshot surfaces ErrInvalidType.
	snapshotBad := config.JSONFromMap(map[string]any{"hooks": "not-a-map"})
	bad := configedit.NewConfig(osFS{}, cwd, snapshotBad)
	_, err = bad.GetMap("hooks").Resolve(ctx)
	assert.ErrorIs(t, err, configedit.ErrInvalidType)
}

func TestEditor_GetRune_reads_snapshot(t *testing.T) {
	_, cwd := dirURI(t)
	snapshot := config.JSONFromMap(map[string]any{
		"separator": "→",
	})
	cfg := configedit.NewConfig(osFS{}, cwd, snapshot)
	ctx := context.Background()

	r, err := cfg.GetRune("separator").Resolve(ctx)
	require.NoError(t, err)
	assert.Equal(t, '→', r)

	_, err = cfg.GetRune("missing").Resolve(ctx)
	assert.ErrorIs(t, err, configedit.ErrNotFound)
}

func TestEditor_GetColor_reads_snapshot(t *testing.T) {
	_, cwd := dirURI(t)
	snapshot := config.JSONFromMap(map[string]any{
		"fg": "red",
	})
	cfg := configedit.NewConfig(osFS{}, cwd, snapshot)
	ctx := context.Background()

	c, err := cfg.GetColor("fg").Resolve(ctx)
	require.NoError(t, err)
	assert.Equal(t, term.ColorRed, c)

	_, err = cfg.GetColor("missing").Resolve(ctx)
	assert.ErrorIs(t, err, configedit.ErrNotFound)
}

func TestEditor_GetAttribute_reads_snapshot(t *testing.T) {
	_, cwd := dirURI(t)
	snapshot := config.JSONFromMap(map[string]any{
		"flag": "bold",
	})
	cfg := configedit.NewConfig(osFS{}, cwd, snapshot)
	ctx := context.Background()

	a, err := cfg.GetAttribute("flag").Resolve(ctx)
	require.NoError(t, err)
	assert.Equal(t, term.AttrBold, a)

	_, err = cfg.GetAttribute("missing").Resolve(ctx)
	assert.ErrorIs(t, err, configedit.ErrNotFound)
}

func TestEditor_Get_returns_ErrNotFound_with_nil_snapshot(t *testing.T) {
	_, cwd := dirURI(t)
	cfg := configedit.NewConfig(osFS{}, cwd, nil)
	ctx := context.Background()

	_, err := cfg.GetMap("x").Resolve(ctx)
	assert.ErrorIs(t, err, configedit.ErrNotFound)
	_, err = cfg.GetRune("x").Resolve(ctx)
	assert.ErrorIs(t, err, configedit.ErrNotFound)
	_, err = cfg.GetColor("x").Resolve(ctx)
	assert.ErrorIs(t, err, configedit.ErrNotFound)
	_, err = cfg.GetAttribute("x").Resolve(ctx)
	assert.ErrorIs(t, err, configedit.ErrNotFound)
}
