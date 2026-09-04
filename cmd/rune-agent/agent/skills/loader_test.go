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

package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeSkill(t *testing.T, dir, name, content string) {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(skillDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(skillDir, "SKILL.md"),
		[]byte(content), 0o644,
	))
}

func TestLoadDir(t *testing.T) {
	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		r := NewRegistry(osFileSystem{}, dirURI(""), []string{dir}, nil)
		// Only builtins should be present, no user skills from the empty dir.
		for _, s := range r.List() {
			assert.NotEqual(t, dir, filepath.Dir(s.Dir),
				"no user skill should come from the empty dir")
		}
	})

	t.Run("nonexistent directory", func(t *testing.T) {
		r := NewRegistry(osFileSystem{}, dirURI(""), []string{"/nonexistent/path"}, nil)
		// Should not panic; only builtins present.
		assert.NotEmpty(t, r.List())
	})

	t.Run("valid skills", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "debug", `---
name: debug
description: Debug issues
---
Check logs`)
		writeSkill(t, dir, "test-repo", `---
name: test-repo
description: Test strategy
---
Run tests`)

		r := NewRegistry(osFileSystem{}, dirURI(""), []string{dir}, nil)
		_, ok := r.Get("debug")
		assert.True(t, ok)
		_, ok = r.Get("test-repo")
		assert.True(t, ok)
	})

	t.Run("invalid SKILL.md skipped", func(t *testing.T) {
		dir := t.TempDir()
		writeSkill(t, dir, "good", `---
name: good
description: Valid skill
---
body`)
		writeSkill(t, dir, "bad", "no frontmatter")

		r := NewRegistry(osFileSystem{}, dirURI(""), []string{dir}, nil)
		_, ok := r.Get("good")
		assert.True(t, ok)
		_, ok = r.Get("bad")
		assert.False(t, ok)
	})

	t.Run("symlinked skill directory", func(t *testing.T) {
		dir := t.TempDir()
		// Create the real skill directory outside the scan directory.
		realDir := t.TempDir()
		writeSkill(t, realDir, "linked", `---
name: linked
description: Symlinked skill
---
linked body`)
		// Symlink it into the scan directory.
		require.NoError(t, os.Symlink(
			filepath.Join(realDir, "linked"),
			filepath.Join(dir, "linked"),
		))

		r := NewRegistry(osFileSystem{}, dirURI(""), []string{dir}, nil)
		s, ok := r.Get("linked")
		assert.True(t, ok, "symlinked skill directory must be discovered")
		assert.Equal(t, "Symlinked skill", s.Description)
	})

	t.Run("subdirectory without SKILL.md skipped", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "empty-dir"), 0o755))
		writeSkill(t, dir, "valid", `---
name: valid
description: A valid skill
---
content`)

		r := NewRegistry(osFileSystem{}, dirURI(""), []string{dir}, nil)
		_, ok := r.Get("valid")
		assert.True(t, ok)
	})
}
