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
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/cmd/rune-agent/configedit"
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
	require.NoError(t, cfg.SetBool(ctx, "force_builtin_tools", false))
	require.NoError(t, cfg.SetInt(ctx, "max_tokens", 2000))

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

	require.NoError(t, cfg.SetBool(ctx, "force_builtin_tools", true))
	require.NoError(t, cfg.SetInt(ctx, "max_tokens", 4096))
	require.NoError(t, cfg.SetString(ctx, "agents_file", "AGENTS.md"))

	content := readConfigFile(t, root)
	assert.Contains(t, content, "force_builtin_tools: true")
	assert.Contains(t, content, "max_tokens: 4096")
	assert.Contains(t, content, "agents_file: AGENTS.md")
	assert.Contains(t, content, "extensions")
	assert.Contains(t, content, "rune-agent")
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

	require.NoError(t, cfg.AppendStringSlice(ctx, "skills", "a/path"))
	require.NoError(t, cfg.AppendStringSlice(ctx, "skills", "b/path"))

	got, err := cfg.GetSlice("skills").Resolve(ctx)
	require.NoError(t, err)
	require.Len(t, got, 2)

	dup := cfg.AppendStringSlice(ctx, "skills", "a/path")
	assert.ErrorIs(t, dup, configedit.ErrAlreadyPresent)

	require.NoError(t, cfg.RemoveStringSlice(ctx, "skills", "a/path"))

	got, err = cfg.GetSlice("skills").Resolve(ctx)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "b/path", got[0])

	miss := cfg.RemoveStringSlice(ctx, "skills", "missing")
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
	assert.NoError(t, cfg.SetBool(ctx, "x", true))
	assert.NoError(t, cfg.AppendStringSlice(ctx, "y", "z"))
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
	assert.Equal(t, tcell.ColorRed, c)

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
	assert.Equal(t, tcell.AttrBold, a)

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
