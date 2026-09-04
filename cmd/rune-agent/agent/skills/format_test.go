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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatSkillContent(t *testing.T) {
	t.Run("basic skill without resources", func(t *testing.T) {
		skill := Skill{
			Name: "commit",
			Body: "Create a git commit.",
			Dir:  "/skills/commit",
		}
		got := FormatSkillContent(skill)
		assert.Contains(t, got, `<skill_content name="commit">`)
		assert.Contains(t, got, "Create a git commit.")
		assert.Contains(t, got, "Skill directory: /skills/commit")
		assert.Contains(t, got, "</skill_content>")
		assert.NotContains(t, got, "<skill_resources>")
	})

	t.Run("skill with resources", func(t *testing.T) {
		dir := t.TempDir()
		scriptsDir := filepath.Join(dir, "scripts")
		require.NoError(t, os.Mkdir(scriptsDir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(scriptsDir, "run.sh"), []byte("#!/bin/sh"), 0o644))

		skill := Skill{
			Name: "deploy",
			Body: "Deploy the app.",
			Dir:  dir,
		}
		got := FormatSkillContent(skill)
		assert.Contains(t, got, `<skill_content name="deploy">`)
		assert.Contains(t, got, "<skill_resources>")
		assert.Contains(t, got, "<file>scripts/run.sh</file>")
		assert.Contains(t, got, "</skill_resources>")
	})
}

func TestEnumerateResources(t *testing.T) {
	t.Run("no resource dirs", func(t *testing.T) {
		dir := t.TempDir()
		files := enumerateResources(dir)
		assert.Empty(t, files)
	})

	t.Run("with files in resource dirs", func(t *testing.T) {
		dir := t.TempDir()
		for _, sub := range []string{"scripts", "references", "assets"} {
			require.NoError(t, os.MkdirAll(filepath.Join(dir, sub), 0o755))
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, "scripts", "a.sh"), nil, 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "assets", "b.txt"), nil, 0o644))
		// subdirectories are skipped
		require.NoError(t, os.Mkdir(filepath.Join(dir, "scripts", "nested"), 0o755))

		files := enumerateResources(dir)
		assert.Len(t, files, 2)
		joined := strings.Join(files, ",")
		assert.Contains(t, joined, "scripts/a.sh")
		assert.Contains(t, joined, "assets/b.txt")
	})
}
