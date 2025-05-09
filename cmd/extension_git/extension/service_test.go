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

package extension

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/sourcegraph/go-diff/diff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/api/schemeapi"
	"unstable.build/go-tui/api/workspaceapi"
	"unstable.build/go-tui/workspace"
)

type gitTestExecutor struct {
	schemeExecutor schemeapi.Executor
}

func (e *gitTestExecutor) Start(cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
	return e.schemeExecutor.StartCommand(context.Background(), cmd)
}

func (e *gitTestExecutor) Signal(pid workspaceapi.Pid, signal syscall.Signal) error {
	return nil
}

func (e *gitTestExecutor) Close() error {
	return nil
}

func setupGitRepos(t *testing.T) (reposPath string) {
	tmpDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	// make tmpDir a canonical path, since usually on macOS is
	// `/var/folders/...` but when you get the repo path using the git cli it
	// returns canonical `/private/var/folders`, so we need to eval symlinks
	tmpDir, err = filepath.EvalSymlinks(tmpDir)
	require.NoError(t, err)

	reposTarball := "testdata/repos.tar"

	cmd := exec.Command("tar", "-xf", reposTarball, "-C", tmpDir)
	err = cmd.Run()
	require.NoError(t, err)

	reposPath = tmpDir + "/repos"

	// mark all projects under tree as safe so git commands work on CI.
	// do it only in the case of CI since locally it works and we don't
	// want to clump our ~/.gitconfig file with many entries to ephemeral
	// directories, if running locally would fail because of that then
	// we would do `git config --global --unset safe.directory ...` in
	// the deferred cleanup step tear down function.
	if os.Getenv("CI") == "true" {
		files, err := os.ReadDir(reposPath)
		require.NoError(t, err)
		for _, file := range files {
			if file.IsDir() {
				cmd = exec.Command(
					"git", "config", "--global", "--add", "safe.directory",
					filepath.Join(reposPath, file.Name()),
				)
				err = cmd.Run()
				require.NoError(t, err)
			}
		}
	}

	return
}

func tearDownGitRepos(reposFolderPath string) {
	os.RemoveAll(reposFolderPath)
}

func setupGitService(t *testing.T, cwd workspaceapi.URI) *cmdGitService {
	scheme, err := workspace.NewFileScheme(
		context.Background(), config.NopConfig(), cwd,
	)
	require.NoError(t, err)

	// gitCliExecutor runs git commands on real repos extracted from tarballs
	gitCliExecutor := new(gitTestExecutor)
	gitCliExecutor.schemeExecutor = scheme

	return newCmdGitService(gitCliExecutor, cwd, scheme)
}

func TestCmdDiff(t *testing.T) {
	reposPath := setupGitRepos(t)
	defer tearDownGitRepos(reposPath)

	tsuite := []struct {
		name         string
		workspaceCwd string
		workPath     string
		expectErr    bool
		assertions   func(t *testing.T, res *diff.FileDiff)
	}{
		{
			name:         "diff folder path within workspace with multiple files changed",
			workspaceCwd: reposPath + "/gitproj3_multi-file-diff",
			workPath:     reposPath + "/gitproj3_multi-file-diff/recipes",
			expectErr:    true,
			// ERROR: file diff reader: read from stdout: line 8, char 333:
			// bad hunk line (does not start with ' ', '-', '+', or '\'): diff
			// --git a/recipes/cucumber-raita.md b/recipes/cucumber-raita.md
			assertions: func(t *testing.T, res *diff.FileDiff) {},
		},
		{
			name:         "diff folder path within workspace with single file changed",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			workPath:     reposPath + "/gitproj2_one-file-diff/recipes/baba-ganoush.md",
			assertions: func(t *testing.T, res *diff.FileDiff) {
				// since only one file in that folder has changes we get very lucky
				assert.Equal(t, "a/recipes/baba-ganoush.md", res.OrigName)
				assert.Len(t, res.Hunks, 1)
			},
		},
		{
			name:         "diff absolute file path within workspace",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			workPath:     reposPath + "/gitproj2_one-file-diff/recipes/baba-ganoush.md",
			assertions: func(t *testing.T, res *diff.FileDiff) {
				assert.Equal(t, "a/recipes/baba-ganoush.md", res.OrigName)
				assert.Len(t, res.Hunks, 1)
				assert.Equal(
					t,
					""+
						"-3. roast the garlic cloves in the broiler or a dry cast iron skillet\n"+
						"+3. Optional: roast the garlic cloves in the broiler or a dry cast iron skillet\n",
					string(res.Hunks[0].Body),
				)
			},
		},
		{
			name:         "diff absolute file path outside workspace",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			workPath:     reposPath + "/gitproj3_multi-file-diff/recipes/cucumber-raita.md",
			assertions: func(t *testing.T, res *diff.FileDiff) {
				assert.Equal(t, "a/recipes/cucumber-raita.md", res.OrigName)
				assert.Len(t, res.Hunks, 2)
			},
		},
		{
			name:         "diff relative file path within workspace",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			workPath:     "./recipes/baba-ganoush.md",
			assertions: func(t *testing.T, res *diff.FileDiff) {
				assert.Equal(t, "a/recipes/baba-ganoush.md", res.OrigName)
				assert.Len(t, res.Hunks, 1)
			},
		},
		{
			name:         "diff relative (no dot ./) file path within workspace",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			workPath:     "recipes/baba-ganoush.md",
			assertions: func(t *testing.T, res *diff.FileDiff) {
				assert.Equal(t, "a/recipes/baba-ganoush.md", res.OrigName)
				assert.Len(t, res.Hunks, 1)
			},
		},
		{
			name:         "diff relative file path outside workspace",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			workPath:     "../gitproj3_multi-file-diff/recipes/cucumber-raita.md",
			assertions: func(t *testing.T, res *diff.FileDiff) {
				assert.Equal(t, "a/recipes/cucumber-raita.md", res.OrigName)
				assert.Len(t, res.Hunks, 2)
			},
		},
		{
			name:         "diff file path no git repo",
			workspaceCwd: reposPath + "/proj4_no-git",
			workPath:     reposPath + "/proj4_no-git/file1.sh",
			expectErr:    true,
			// ERROR: rel path: repo path: git cmd: process exit with non-zero
			// status (exit status 128) fatal: not a git repository (or any of
			// the parent director
			assertions: func(t *testing.T, res *diff.FileDiff) {},
		},
		{
			name:         "repoless absolute path above cwd repo",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			workPath:     reposPath + "/top-level-file-sibling-to-repos.txt",
			expectErr:    true,
			// ERROR: rel path: repo path: git cmd: process exit with non-zero
			// status (exit status 128) fatal: not a git repository (or any of
			// the parent director
			assertions: func(t *testing.T, res *diff.FileDiff) {},
		},
		{
			name:         "repoless relative path above cwd repo",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			workPath:     "../top-level-file-sibling-to-repos.txt",
			expectErr:    true,
			// ERROR: rel path: repo path: git cmd: process exit with non-zero
			// status (exit status 128) fatal: not a git repository (or any of
			// the parent director
			assertions: func(t *testing.T, res *diff.FileDiff) {},
		},
		{
			name:         "diff relative file path without changes",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			workPath:     "./README.txt",
			expectErr:    true,
			// ERROR (`errDiffNoChanges`): diff no changes
			assertions: func(t *testing.T, res *diff.FileDiff) {},
		},
		{
			name:         "non-existent absolute file",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			workPath:     reposPath + "/this-file-does-not-exist.lol",
			expectErr:    true,
			// ERROR: rel path: repo path: git cmd: process exit with non-zero
			// status (exit status 128) fatal: not a git repository (or any of
			// the parent director
			assertions: func(t *testing.T, res *diff.FileDiff) {},
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			testServiceFunction(t,
				tcase.workspaceCwd,
				func(git *cmdGitService) (res *diff.FileDiff, err error) {
					return git.diff(tcase.workPath)
				},
				tcase.expectErr, tcase.assertions)
		})
	}
}

func TestCmdGitCurrentCommit(t *testing.T) {
	reposPath := setupGitRepos(t)
	defer tearDownGitRepos(reposPath)

	tsuite := []struct {
		name         string
		workspaceCwd string
		workPath     string
		expectErr    bool
		expect       string
	}{
		{
			name:         "repoless workPath",
			workspaceCwd: reposPath + "/proj4_no-git",
			workPath:     reposPath + "/proj4_no-git",
			expectErr:    true,
			// ERROR: rel path: repo path: git cmd: process exit with non-zero status
			// (exit status 128) fatal: not a git repository (or any of the parent
			// directories): .git
		},
		{
			name:         "workPath absolute folder within cwd",
			workspaceCwd: reposPath + "/gitproj1_work-dir-clean",
			workPath:     reposPath + "/gitproj1_work-dir-clean",
			expect:       "b8922f92b6223af3a548d1904d1825774fb3b20f",
		},
		{
			name:         "workPath relative folder within cwd",
			workspaceCwd: reposPath + "/gitproj3_multi-file-diff",
			workPath:     "./recipes/",
			expect:       "368043d01e0fb3c543b28ea4f422208fcbc40b32",
		},
		{
			name:         "workPath relative folder (no dot ./) within cwd",
			workspaceCwd: reposPath + "/gitproj3_multi-file-diff",
			workPath:     "./recipes/",
			expect:       "368043d01e0fb3c543b28ea4f422208fcbc40b32",
		},
		{
			name:         "workPath absolute file within cwd",
			workspaceCwd: reposPath + "/gitproj3_multi-file-diff",
			workPath:     reposPath + "/gitproj3_multi-file-diff/recipes/bagels.md",
			expect:       "368043d01e0fb3c543b28ea4f422208fcbc40b32",
		},
		{
			name:         "workPath relative file within cwd",
			workspaceCwd: reposPath + "/gitproj3_multi-file-diff",
			workPath:     "./recipes/bagels.md",
			expect:       "368043d01e0fb3c543b28ea4f422208fcbc40b32",
		},
		{
			name:         "workPath relative file repo outside cwd",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			workPath:     "../gitproj3_multi-file-diff/recipes/bagels.md",
			expect:       "368043d01e0fb3c543b28ea4f422208fcbc40b32",
		},
	}
	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			testServiceFunction(t,
				tcase.workspaceCwd,
				func(git *cmdGitService) (res string, err error) {
					return git.currentCommit(tcase.workPath)
				},
				tcase.expectErr,
				func(t *testing.T, res string) {
					assert.Equal(t, tcase.expect, res)
				})
		})
	}
}

func TestCmdGitRemoteURL(t *testing.T) {
	reposPath := setupGitRepos(t)
	defer tearDownGitRepos(reposPath)

	tsuite := []struct {
		name         string
		workspaceCwd string
		workPath     string
		remoteName   string
		expectErr    bool
		expect       string
	}{
		{
			name:         "repoless workPath",
			workspaceCwd: reposPath + "/proj4_no-git",
			workPath:     reposPath + "/proj4_no-git",
			expectErr:    true,
			// ERROR: process exit with non-zero status (exit status 128)
			// fatal: not a git repository (or any of the parent directories):
			// .git
			remoteName: "origin",
		},
		{
			name:         "remote exists",
			workspaceCwd: reposPath + "/gitproj6_two-remotes",
			workPath:     reposPath + "/gitproj6_two-remotes",
			remoteName:   "origin",
			expect:       "git@git.unstable.build:unstablebuild/gitproj6.git",
		},
		{
			name:         "empty remote name",
			workspaceCwd: reposPath + "/gitproj6_two-remotes",
			workPath:     reposPath + "/gitproj6_two-remotes",
			expectErr:    true,
			// ERROR: must pass remote name
		},
		{
			name:         "inexistent remote name",
			workspaceCwd: reposPath + "/gitproj6_two-remotes",
			workPath:     reposPath + "/gitproj6_two-remotes",
			remoteName:   "unexistent-remote-name",
			expectErr:    true,
			// ERROR: process exit with non-zero status (exit status 2) error:
			// No such remote 'unexistent-remote-name'
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			testServiceFunction(t,
				tcase.workspaceCwd,
				func(git *cmdGitService) (res string, err error) {
					return git.remoteURL(tcase.workPath, tcase.remoteName)
				},
				tcase.expectErr,
				func(t *testing.T, res string) {
					assert.Equal(t, tcase.expect, res)
				})
		})
	}
}

func TestCmdRepoPath(t *testing.T) {
	reposPath := setupGitRepos(t)
	defer tearDownGitRepos(reposPath)

	tsuite := []struct {
		name         string
		workspaceCwd string
		file         string
		expectErr    bool
		expect       string
	}{
		{
			name:         "absolute file not a repo",
			workspaceCwd: reposPath,
			file:         reposPath + "/top-level-file-sibling-to-repos.txt",
			expectErr:    true,
			// ERROR: repo path: git cmd: process exit with non-zero status
			// (exit status 128) fatal: not a git repository (or any of the
			// parent directories): .git
		},
		{
			name:         "absolute file within repo same cwd",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			file:         reposPath + "/gitproj2_one-file-diff/recipes/baba-ganoush.md",
			expect:       reposPath + "/gitproj2_one-file-diff",
		},
		{
			name:         "absolute file in other repo not below cwd",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			file:         reposPath + "/gitproj3_multi-file-diff/recipes/cucumber-raita.md",
			expect:       reposPath + "/gitproj3_multi-file-diff",
		},
		{
			name:         "absolute file within repo below cwd",
			workspaceCwd: reposPath,
			file:         reposPath + "/gitproj2_one-file-diff/recipes/baba-ganoush.md",
			expect:       reposPath + "/gitproj2_one-file-diff",
		},
		{
			name:         "absolute file within repo above cwd",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff/recipes",
			file:         reposPath + "/gitproj2_one-file-diff/recipes/baba-ganoush.md",
			expect:       reposPath + "/gitproj2_one-file-diff",
		},
		{
			name:         "relative file within repo same level cwd",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			file:         "./recipes/baba-ganoush.md",
			expect:       reposPath + "/gitproj2_one-file-diff",
		},
		{
			name:         "relative file (no dot ./) within repo same level cwd",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			file:         "recipes/baba-ganoush.md",
			expect:       reposPath + "/gitproj2_one-file-diff",
		},
		{
			name:         "relative file within repo below cwd",
			workspaceCwd: reposPath,
			file:         "./gitproj2_one-file-diff/recipes/baba-ganoush.md",
			expect:       reposPath + "/gitproj2_one-file-diff",
		},
		{
			name:         "relative file within repo above cwd",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff/recipes",
			file:         "../README.txt",
			expect:       reposPath + "/gitproj2_one-file-diff",
		},
		{
			name:         "relative file in other repo sibling to cwd",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff/recipes",
			file:         "../../gitproj3_multi-file-diff/recipes/cucumber-raita.md",
			expect:       reposPath + "/gitproj3_multi-file-diff",
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			testServiceFunction(t,
				tcase.workspaceCwd,
				func(git *cmdGitService) (res string, err error) {
					return git.repoPath(tcase.file)
				},
				tcase.expectErr,
				func(t *testing.T, res string) {
					assert.Equal(t, tcase.expect, res)
				})
		})
	}
}

func TestCmdRelPath(t *testing.T) {
	reposPath := setupGitRepos(t)
	defer tearDownGitRepos(reposPath)

	tsuite := []struct {
		name         string
		workspaceCwd string
		file         string
		expectErr    bool
		expect       string
	}{
		{
			name:         "absolute file not a repo",
			workspaceCwd: reposPath,
			file:         reposPath + "/top-level-file-sibling-to-repos.txt",
			expectErr:    true,
			// ERROR: repo path: git cmd: process exit with non-zero status
			// (exit status 128) fatal: not a git repository (or any of the
			// parent directories): .git
		},
		{
			name:         "absolute file within repo same cwd",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			file:         reposPath + "/gitproj2_one-file-diff/recipes/baba-ganoush.md",
			expect:       "recipes/baba-ganoush.md",
		},
		{
			name:         "absolute file in other repo not below cwd",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			file:         reposPath + "/gitproj3_multi-file-diff/recipes/cucumber-raita.md",
			expect:       "recipes/cucumber-raita.md",
		},
		{
			name:         "absolute file within repo below cwd",
			workspaceCwd: reposPath,
			file:         reposPath + "/gitproj2_one-file-diff/recipes/baba-ganoush.md",
			expect:       "recipes/baba-ganoush.md",
		},
		{
			name:         "absolute file within repo above cwd",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff/recipes",
			file:         reposPath + "/gitproj2_one-file-diff/recipes/baba-ganoush.md",
			expect:       "recipes/baba-ganoush.md",
		},
		{
			name:         "relative file within repo same level cwd",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			file:         "./recipes/baba-ganoush.md",
			expect:       "recipes/baba-ganoush.md",
		},
		{
			name:         "relative file (no dot ./) within repo same level cwd",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			file:         "recipes/baba-ganoush.md",
			expect:       "recipes/baba-ganoush.md",
		},
		{
			name:         "relative file within repo below cwd",
			workspaceCwd: reposPath,
			file:         "./gitproj2_one-file-diff/recipes/baba-ganoush.md",
			expect:       "recipes/baba-ganoush.md",
		},
		{
			name:         "relative file within repo above cwd",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff/recipes",
			file:         "../README.txt",
			expect:       "README.txt",
		},
		{
			name:         "relative file in other repo sibling to cwd",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff/recipes",
			file:         "../../gitproj3_multi-file-diff/recipes/cucumber-raita.md",
			expect:       "recipes/cucumber-raita.md",
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			testServiceFunction(t,
				tcase.workspaceCwd,
				func(git *cmdGitService) (res string, err error) {
					return git.relPath(tcase.file)
				},
				tcase.expectErr,
				func(t *testing.T, res string) {
					assert.Equal(t, tcase.expect, res)
				})
		})
	}
}

func testServiceFunction[T any](
	t *testing.T, workspaceCwd string,
	serviceFn func(git *cmdGitService) (res T, err error),
	expectErr bool,
	assertions func(t *testing.T, res T),
) {
	workspaceCwdURI, err := workspaceapi.ParseURI("file://" + workspaceCwd)
	require.NoError(t, err)

	git := setupGitService(t, workspaceCwdURI)

	res, err := serviceFn(git)
	if expectErr {
		require.Error(t, err)
	} else {
		require.NoError(t, err)
	}

	assertions(t, res)
}
