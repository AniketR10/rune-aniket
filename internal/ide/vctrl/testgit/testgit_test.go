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

package testgit

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}
}

func TestEnvStripsInheritedGitVars(t *testing.T) {
	// Plant some GIT_* vars in the parent env. t.Setenv restores them
	// when the test exits.
	t.Setenv("GIT_DIR", "/should/not/leak")
	t.Setenv("GIT_WORK_TREE", "/should/not/leak")
	t.Setenv("GIT_INDEX_FILE", "/should/not/leak")

	dir := t.TempDir()
	env := Env(dir)

	for _, kv := range env {
		if strings.HasPrefix(kv, "GIT_DIR=") ||
			strings.HasPrefix(kv, "GIT_WORK_TREE=") ||
			strings.HasPrefix(kv, "GIT_INDEX_FILE=") {
			t.Fatalf("Env leaked inherited git var: %q", kv)
		}
	}

	// Sanity: hardened vars are present.
	want := []string{
		"HOME=" + dir,
		"XDG_CONFIG_HOME=" + dir,
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_CEILING_DIRECTORIES=" + filepath.Dir(dir),
		"GIT_AUTHOR_NAME=test",
	}
	for _, w := range want {
		assert.Contains(t, env, w, "expected hardened var %q", w)
	}
}

func TestRunInitCreatesGitDir(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	Run(t, dir, "init", "-q")

	info, err := os.Stat(filepath.Join(dir, ".git"))
	require.NoError(t, err)
	assert.True(t, info.IsDir(), ".git should be a directory")
}

func TestCeilingPreventsUpwardDiscovery(t *testing.T) {
	requireGit(t)

	// Create a parent repo and a sibling directory that is NOT inside it.
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	require.NoError(t, os.Mkdir(repo, 0o755))
	Run(t, repo, "init", "-q")

	sibling := filepath.Join(parent, "sibling")
	require.NoError(t, os.Mkdir(sibling, 0o755))

	// From the sibling, git must not discover any repo. GIT_CEILING_DIRECTORIES
	// is set to filepath.Dir(sibling)==parent, so discovery stops there.
	cmd := Command(t, sibling, "rev-parse", "--show-toplevel")
	out, err := cmd.CombinedOutput()
	assert.Error(t, err,
		"git should fail in sibling dir; got output: %s", out)
}
