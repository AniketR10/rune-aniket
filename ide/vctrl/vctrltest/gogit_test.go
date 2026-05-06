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
	"strings"
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

func TestGogitDiffCrossWorkspace(t *testing.T) {
	// Regression test for RUNE-8: opening a file from a different workspace
	// must not diff it against the current workspace repository when the
	// files share the same repo-relative path.
	tmpDir, err := os.MkdirTemp("", "cross-workspace-*")
	require.NoError(t, err)
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	repoA := filepath.Join(tmpDir, "repoA")
	repoB := filepath.Join(tmpDir, "repoB")
	sharedRelPath := filepath.Join("src", "main.txt")

	// Initialize repo A with src/main.txt containing "alpha"
	initGitRepo(t, repoA, sharedRelPath, "alpha\n")
	// Initialize repo B with src/main.txt containing "beta"
	initGitRepo(t, repoB, sharedRelPath, "beta\n")

	// Modify the working copy in repo B so there is a diff against B's HEAD
	modifiedContent := "beta\nmodified in workspace B\n"
	err = os.WriteFile(filepath.Join(repoB, sharedRelPath), []byte(modifiedContent), 0644)
	require.NoError(t, err)

	// Set up the service with repo A as the workspace
	workspaceURI, err := workspaceapi.ParseURI("file://" + repoA)
	require.NoError(t, err)
	svc := setupGogitService(t, workspaceURI)

	// Diff a file from repo B while the workspace is repo A
	fileURI, err := workspaceapi.ParseURI("file://" + filepath.Join(repoB, sharedRelPath))
	require.NoError(t, err)

	res, err := svc.Diff(context.Background(), fileURI)
	require.NoError(t, err, "Diff should succeed for a file in a different workspace repo")

	// The diff must reflect changes relative to repo B's HEAD ("beta\n"),
	// NOT repo A's HEAD ("alpha\n"). If the service incorrectly uses
	// repo A's committed content, the diff would show removal of "alpha"
	// and addition of the modified content, which is wrong.
	require.NotZero(t, len(res.Hunks), "should detect changes in the file")

	// Collect all inserted/deleted lines from the hunk bodies.
	// Each body line is prefixed with '+' (added), '-' (removed), or ' ' (context).
	var addedLines, removedLines string
	for _, hunk := range res.Hunks {
		for _, line := range strings.Split(hunk.Body, "\n") {
			if strings.HasPrefix(line, "+") {
				addedLines += line[1:] + "\n"
			} else if strings.HasPrefix(line, "-") {
				removedLines += line[1:] + "\n"
			}
		}
	}

	// Correct diff (against repo B HEAD "beta\n") should add the
	// modification line, and should NOT remove "alpha" (which only exists
	// in repo A).
	assert.NotContains(t, removedLines, "alpha",
		"diff must not reference repo A content; should diff against repo B's HEAD")
	assert.Contains(t, addedLines, "modified in workspace B",
		"diff should show the working copy change relative to repo B HEAD")
}

func TestGogitDiffSameWorkspace(t *testing.T) {
	// Ensure the fast path for files in the active workspace repo still works.
	reposPath := setupGitRepos(t)

	workspaceCwd := reposPath + "/gitproj2_one-file-diff"
	workspaceCwdURI, err := workspaceapi.ParseURI("file://" + workspaceCwd)
	require.NoError(t, err)
	svc := setupGogitService(t, workspaceCwdURI)

	filePath := workspaceCwd + "/recipes/baba-ganoush.md"
	uri, err := workspaceapi.ParseURI("file://" + filePath)
	require.NoError(t, err)
	res, err := svc.Diff(context.Background(), uri)
	require.NoError(t, err)
	assert.NotZero(t, res.Hunks, "should detect changes in the workspace file")
	assert.Equal(t, "baba-ganoush.md", filepath.Base(res.OrigName))
}

func TestGogitDiffSurvivesExternalRepack(t *testing.T) {
	// Regression test for RUNE-133: when external git operations rewrite
	// packs (e.g. `git gc`), the gogit service must not return
	// `get HEAD commit object: object not found` because of a stale
	// in-process pack index cache.
	tmpDir, err := os.MkdirTemp("", "gogit-repack-*")
	require.NoError(t, err)
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tmpDir) })

	repo := filepath.Join(tmpDir, "repo")
	relPath := "a.txt"
	initGitRepo(t, repo, relPath, "hi\n")

	// Initial aggressive gc to ensure objects start out packed.
	runGit(t, repo, "gc", "--aggressive", "--prune=now")

	workspaceURI, err := workspaceapi.ParseURI("file://" + repo)
	require.NoError(t, err)
	svc := setupGogitService(t, workspaceURI)

	fileURI, err := workspaceapi.ParseURI("file://" + filepath.Join(repo, relPath))
	require.NoError(t, err)

	// First diff: warms the cached storage / pack index.
	require.NoError(t, os.WriteFile(filepath.Join(repo, relPath), []byte("hi\nworld\n"), 0644))
	first, err := svc.Diff(context.Background(), fileURI)
	require.NoError(t, err, "initial diff should succeed")
	require.NotZero(t, len(first.Hunks), "initial diff should detect changes")

	// Out-of-band repo mutation: commit and aggressively repack so the
	// previously-cached pack hashes no longer exist on disk.
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "c2")
	runGit(t, repo, "gc", "--aggressive", "--prune=now")

	// Modify the working copy and diff again. With a stale cached repo
	// this fails with `get HEAD commit object: object not found`.
	require.NoError(t, os.WriteFile(filepath.Join(repo, relPath), []byte("hi\nworld\nmore\n"), 0644))
	second, err := svc.Diff(context.Background(), fileURI)
	require.NoError(t, err, "diff after external repack should succeed")
	assert.NotZero(t, len(second.Hunks), "diff after external repack should detect changes")
}

// runGit runs a git subcommand in dir with deterministic author/committer.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

// initGitRepo creates a git repo at root with a single committed file.
func initGitRepo(t *testing.T, root, relPath, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.Dir(relPath)), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(root, relPath), []byte(content), 0644))

	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"add", "."},
		{"commit", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test",
			"GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test",
			"GIT_COMMITTER_EMAIL=test@test.com",
		)
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
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
