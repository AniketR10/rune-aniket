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
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

func TestBootstrap(t *testing.T) {
	t.Run("creates workspace files on empty directory", func(t *testing.T) {
		dir := t.TempDir()
		fsys := newOSFileSystem()

		result, err := bootstrap(fsys, dir)
		require.NoError(t, err)
		assert.Equal(t, actionCreated, result.Action)

		for _, name := range []string{"go.mod", "categories.go", "main.go", "main_test.go", "conversation.go", "claude.go", "version"} {
			_, err := os.Stat(filepath.Join(dir, name))
			assert.NoError(t, err, "expected %s to exist", name)
		}

		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		require.NoError(t, err)
		assert.Contains(t, string(data), "module memories")
	})

	t.Run("skips when go.mod exists and version is current", func(t *testing.T) {
		dir := t.TempDir()
		fsys := newOSFileSystem()

		// Bootstrap first.
		_, err := bootstrap(fsys, dir)
		require.NoError(t, err)

		// Second call should be a no-op.
		result, err := bootstrap(fsys, dir)
		require.NoError(t, err)
		assert.Equal(t, actionNoop, result.Action)
	})

	t.Run("upgrades when version is stale", func(t *testing.T) {
		dir := t.TempDir()
		fsys := newOSFileSystem()

		// Create go.mod with stale version.
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module memories\n\ngo 1.25.6\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "version"), []byte("1\n"), 0o644))

		result, err := bootstrap(fsys, dir)
		require.NoError(t, err)
		assert.Equal(t, actionUpgraded, result.Action)

		// go.mod should be overwritten with the new template.
		data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		require.NoError(t, err)
		assert.Contains(t, string(data), "rune-go-sdk")

		// Conversation helper should exist.
		_, err = os.Stat(filepath.Join(dir, "conversation.go"))
		assert.NoError(t, err)
	})

	t.Run("upgrades when version file is missing", func(t *testing.T) {
		dir := t.TempDir()
		fsys := newOSFileSystem()

		// Create go.mod without version file (simulates pre-versioning workspace).
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module memories\n\ngo 1.25.6\n"), 0o644))

		result, err := bootstrap(fsys, dir)
		require.NoError(t, err)
		assert.Equal(t, actionUpgraded, result.Action)
	})

	t.Run("creates parent directories", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "a", "b", "c")
		fsys := newOSFileSystem()

		result, err := bootstrap(fsys, dir)
		require.NoError(t, err)
		assert.Equal(t, actionCreated, result.Action)

		_, err = os.Stat(filepath.Join(dir, "go.mod"))
		assert.NoError(t, err)
	})

	t.Run("carries fromVersion on upgrade", func(t *testing.T) {
		dir := t.TempDir()
		fsys := newOSFileSystem()

		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module memories\n\ngo 1.25.6\n"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "version"), []byte("1\n"), 0o644))

		result, err := bootstrap(fsys, dir)
		require.NoError(t, err)
		assert.Equal(t, actionUpgraded, result.Action)
		assert.Equal(t, 1, result.FromVersion)
	})

	t.Run("fromVersion is zero on fresh create", func(t *testing.T) {
		dir := t.TempDir()
		fsys := newOSFileSystem()

		result, err := bootstrap(fsys, dir)
		require.NoError(t, err)
		assert.Equal(t, actionCreated, result.Action)
		assert.Equal(t, 0, result.FromVersion)
	})

	t.Run("claude.go included in bootstrap", func(t *testing.T) {
		dir := t.TempDir()
		fsys := newOSFileSystem()

		_, err := bootstrap(fsys, dir)
		require.NoError(t, err)

		convData, readErr := os.ReadFile(filepath.Join(dir, "conversation.go"))
		require.NoError(t, readErr)
		assert.Contains(t, string(convData), "FetchDialogue")

		claudeData, readErr := os.ReadFile(filepath.Join(dir, "claude.go"))
		require.NoError(t, readErr)
		assert.Contains(t, string(claudeData), "FetchClaudeDialogue")
	})

	t.Run("bootstrapped workspace compiles", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		dir := t.TempDir()
		fsys := newOSFileSystem()
		_, err := bootstrap(fsys, dir)
		require.NoError(t, err)

		assertGoCommand(t, dir, "mod", "tidy")
		assertGoCommand(t, dir, "build", "./...")
	})

	t.Run("bootstrapped workspace tests pass", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping integration test in short mode")
		}
		dir := t.TempDir()
		fsys := newOSFileSystem()
		_, err := bootstrap(fsys, dir)
		require.NoError(t, err)

		assertGoCommand(t, dir, "mod", "tidy")
		assertGoCommand(t, dir, "test", "./...")
	})
}

func assertGoCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("go", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %v failed:\n%s", args, out)
	}
}

// osFileSystem adapts the real OS to workspaceapi.FileSystem for testing.
type osFileSystem struct{}

func newOSFileSystem() workspaceapi.FileSystem {
	return osFileSystem{}
}

func (osFileSystem) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.CurrentUserHostURI(path)
}

func (osFileSystem) OpenFile(path string, flag int, mode os.FileMode) (workspaceapi.File, error) {
	return os.OpenFile(path, flag, mode)
}

func (osFileSystem) Remove(path string) error                     { return os.Remove(path) }
func (osFileSystem) Stat(path string) (os.FileInfo, error)        { return os.Stat(path) }
func (osFileSystem) ReadDir(name string) ([]os.DirEntry, error)   { return os.ReadDir(name) }
func (osFileSystem) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
