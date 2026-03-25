// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package vctrltest

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/ide/vctrl"
	"unstable.build/go-tui/ide/vctrl/gogit"
	"unstable.build/go-tui/workspace"
)

func TestGogitDiff(t *testing.T) {
	testGitDiff(t, setupGogitService)
}

func TestGogitGitCurrentCommit(t *testing.T) {
	testGitCurrentCommit(t, setupGogitService)
}

func TestGogitGitShortRef(t *testing.T) {
	testGitShortRef(t, setupGogitService)
}

func TestGogitGitRemoteURL(t *testing.T) {
	testGitRemoteURL(t, setupGogitService)
}

func TestGogitRelPath(t *testing.T) {
	testRelPath(t, setupGogitService)
}

func TestGogitDiffWorktree(t *testing.T) {
	reposPath := setupGitRepos(t)

	// Use gitproj2_one-file-diff as the main repo — it has a known diff
	// in recipes/baba-ganoush.md
	mainRepoPath := filepath.Join(reposPath, "gitproj2_one-file-diff")

	// Create a worktree from the main repo
	worktreePath := filepath.Join(reposPath, "worktree1")
	cmd := exec.Command("git", "worktree", "add", worktreePath, "HEAD")
	cmd.Dir = mainRepoPath
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git worktree add: %s", out)

	// Verify .git is a file (not a directory) in the worktree
	info, err := os.Stat(filepath.Join(worktreePath, ".git"))
	require.NoError(t, err)
	require.False(t, info.IsDir(), ".git in worktree should be a file, not a directory")

	// Modify a file in the worktree to create a diff
	targetFile := filepath.Join(worktreePath, "recipes", "baba-ganoush.md")
	original, err := os.ReadFile(targetFile)
	require.NoError(t, err)
	err = os.WriteFile(targetFile, append(original, []byte("\n// worktree modification\n")...), 0644)
	require.NoError(t, err)

	// Setup the gogit service pointing at the worktree
	worktreeCwdURI, err := workspaceapi.ParseURI("file://" + worktreePath)
	require.NoError(t, err)
	svc := setupGogitService(t, worktreeCwdURI)

	// Diff should work on the worktree file
	uri, err := workspaceapi.ParseURI("file://" + targetFile)
	require.NoError(t, err)
	res, err := svc.Diff(context.Background(), uri)
	require.NoError(t, err, "Diff should work on files in a git worktree")
	assert.NotZero(t, res.Hunks, "should have detected the worktree modification")
}

func setupGogitService(t *testing.T, cwd workspaceapi.URI) vctrl.Service {
	scheme, err := workspace.NewFileScheme(
		context.Background(), config.NopConfig(), cwd,
	)
	require.NoError(t, err)

	svc, err := gogit.NewService(cwd, scheme)
	require.NoError(t, err)
	return svc
}
