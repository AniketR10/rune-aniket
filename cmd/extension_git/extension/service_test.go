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
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
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

func setupGitService(t *testing.T, cwd workspaceapi.URI) gitService {
	scheme, err := workspace.NewFileScheme(
		context.Background(), config.NopConfig(), cwd,
	)
	require.NoError(t, err)

	// gitCliExecutor runs git commands on real repos extracted from tarballs
	gitCliExecutor := new(gitTestExecutor)
	gitCliExecutor.schemeExecutor = scheme

	return newCmdGitService(gitCliExecutor)
}

func TestCmdGitDiff(t *testing.T) {
	reposPath := setupGitRepos(t)
	defer tearDownGitRepos(reposPath)

	tsuite := []struct {
		name           string
		expectErr      bool
		chDir          string
		workspaceCwd   string
		diffFilePath   string
		diffAssertions func(*diff.FileDiff)
	}{
		// -- TEST git diff on folders
		{
			name:      "diff folder path within workspace with multiple files changed",
			expectErr: true,
			// ERROR: file diff reader: read from stdout: line 8, char 333:
			// bad hunk line (does not start with ' ', '-', '+', or '\'): diff
			// --git a/recipes/cucumber-raita.md b/recipes/cucumber-raita.md
			chDir:          reposPath + "/gitproj3_multi-file-diff",
			workspaceCwd:   reposPath + "/gitproj3_multi-file-diff",
			diffFilePath:   reposPath + "/gitproj3_multi-file-diff/recipes",
			diffAssertions: func(res *diff.FileDiff) {},
		},
		{
			name:         "diff folder path within workspace with single file changed",
			chDir:        reposPath + "/gitproj2_one-file-diff",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			diffFilePath: reposPath + "/gitproj2_one-file-diff/recipes/baba-ganoush.md",
			diffAssertions: func(res *diff.FileDiff) {
				// since only one file in that folder has changes we get very lucky
				assert.Equal(t, "a/recipes/baba-ganoush.md", res.OrigName)
				assert.Len(t, res.Hunks, 1)
			},
		},
		// -- TEST git diff on files
		{
			name:         "diff absolute file path within workspace",
			chDir:        reposPath + "/gitproj2_one-file-diff",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			diffFilePath: reposPath + "/gitproj2_one-file-diff/recipes/baba-ganoush.md",
			diffAssertions: func(res *diff.FileDiff) {
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
			name:      "diff absolute file path outside workspace",
			expectErr: true,
			// ERROR: git cmd: process exit with non-zero status (exit status
			// 128) fatal: <tmpDir>/repos/gitproj3_multi-file-diff/recipes:
			// '<tmpDir>/repos/gitproj3_multi-file-diff/recipes' is outside
			// repository at '<tmpDir>/repos/gitproj2_one-file-diff'
			chDir:          reposPath + "/gitproj2_one-file-diff",
			workspaceCwd:   reposPath + "/gitproj2_one-file-diff",
			diffFilePath:   reposPath + "/gitproj3_multi-file-diff/recipes/cucumber-raita.md",
			diffAssertions: func(res *diff.FileDiff) {},
		},
		{
			name:         "diff relative file path within workspace",
			chDir:        reposPath + "/gitproj2_one-file-diff",
			workspaceCwd: reposPath + "/gitproj2_one-file-diff",
			diffFilePath: "./recipes/baba-ganoush.md",
			diffAssertions: func(res *diff.FileDiff) {
				assert.Equal(t, "a/recipes/baba-ganoush.md", res.OrigName)
				assert.Len(t, res.Hunks, 1)
			},
		},
		{
			name:      "diff relative file path outside workspace",
			expectErr: true,
			// ERROR: git cmd: process exit with non-zero status (exit status
			// 128) fatal: <tmpDir>/repos/gitproj3_multi-file-diff/recipes:
			// '<tmpDir>/repos/gitproj3_multi-file-diff/recipes' is outside
			// repository at '<tmpDir>/repos/gitproj2_one-file-diff'

			chDir:          reposPath + "/gitproj2_one-file-diff",
			workspaceCwd:   reposPath + "/gitproj2_one-file-diff",
			diffFilePath:   "../gitproj3_multi-file-diff/recipes/cucumber-raita.md",
			diffAssertions: func(res *diff.FileDiff) {},
		},
		{
			name:      "diff file path no git repo",
			expectErr: true,
			// ERROR: git cmd: process exit with non-zero status (exit status
			// 129) warning: Not a git repository. [...]
			chDir:          reposPath + "/proj4_no-git",
			workspaceCwd:   reposPath + "/proj4_no-git",
			diffFilePath:   reposPath + "/proj4_no-git/file1.sh",
			diffAssertions: func(res *diff.FileDiff) {},
		},
		{
			name:      "git-less absolute path above cwd repo",
			expectErr: true,
			// ERROR: git cmd: process exit with non-zero status (exit status
			// 128) fatal: <tmpDir>/repos/top-level-file-sibling-to-repos.txt:
			// '<tmpDir>/repos/top-level-file-sibling-to-repos.txt' is outside
			// repository at '/<tmpDir>/repos/gitproj2_one-file-diff'
			chDir:          reposPath + "/gitproj2_one-file-diff",
			workspaceCwd:   reposPath + "/gitproj2_one-file-diff",
			diffFilePath:   reposPath + "/top-level-file-sibling-to-repos.txt",
			diffAssertions: func(res *diff.FileDiff) {},
		},
		{
			name:      "git-less relative path above cwd repo",
			expectErr: true,
			// ERROR: git cmd: process exit with non-zero status (exit status
			// 128) fatal: ambiguous argument
			// '../top-level-file-sibling-to-repos.txt': unknown revision or
			// path not in the working tree.
			chDir:          reposPath + "/gitproj2_one-file-diff",
			workspaceCwd:   reposPath + "/gitproj2_one-file-diff",
			diffFilePath:   "../top-level-file-sibling-to-repos.txt",
			diffAssertions: func(res *diff.FileDiff) {},
		},
		{
			name:      "diff relative file path without changes",
			expectErr: true,
			// ERROR (`errDiffNoChanges`): diff no changes
			chDir:          reposPath + "/gitproj2_one-file-diff",
			workspaceCwd:   reposPath + "/gitproj2_one-file-diff",
			diffFilePath:   "./README.txt",
			diffAssertions: func(res *diff.FileDiff) {},
		},
		{
			name:      "non-existent file",
			expectErr: true,
			// ERROR: git cmd: process exit with non-zero status (exit status
			// 128) fatal: ambiguous argument 'this-file-does-not-exist.lol':
			// unknown revision or path not in the working tree. [...]
			chDir:          reposPath + "/gitproj2_one-file-diff",
			workspaceCwd:   reposPath + "/gitproj2_one-file-diff",
			diffFilePath:   "this-file-does-not-exist.lol",
			diffAssertions: func(res *diff.FileDiff) {},
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			cwdBefore, err := os.Getwd()
			require.NoError(t, err)
			defer os.Chdir(cwdBefore)

			if tcase.chDir != "" {
				err := os.Chdir(tcase.chDir)
				require.NoError(t, err)
			}

			workspaceCwdURI, err := workspaceapi.ParseURI("file://" + tcase.workspaceCwd)
			require.NoError(t, err)

			git := setupGitService(t, workspaceCwdURI)

			res, err := git.diff(tcase.diffFilePath)
			if tcase.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			tcase.diffAssertions(res)
		})
	}
}
