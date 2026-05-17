// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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
