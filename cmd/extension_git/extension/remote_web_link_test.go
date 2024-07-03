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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

func TestRemoteWebLink(t *testing.T) {
	reposPath := setupGitRepos(t)
	defer tearDownGitRepos(reposPath)

	tsuite := []struct {
		name         string
		mustError    bool
		workspaceCwd string
		remoteName   string
		inputFile    string
		inputLine    int
		expect       string
	}{
		{
			name:         "file from repo within cwd and line",
			workspaceCwd: reposPath + "/gitproj6_two-remotes",
			remoteName:   "origin",
			inputFile:    reposPath + "/gitproj6_two-remotes/recipes/guasacaca.md",
			inputLine:    32,
			expect: "https://git.unstable.build/unstablebuild/gitproj6" +
				"/src/commit/5367f818c4e7092a224ae0f32da1b7bab8843cc0/recipes/guasacaca.md#L33",
		},
		{
			name:         "absolute file from repo on other tree not in cwd",
			workspaceCwd: reposPath + "/gitproj6_two-remotes",
			remoteName:   "origin",
			inputFile:    reposPath + "/gitproj5_one-remote/recipes/chile-colorado.md",
			inputLine:    7,
			expect: "https://git.unstable.build/unstablebuild/gitproj5" +
				"/src/commit/b0c7a3e628f4bfb0cb60f2ee048b7e47017c57e7/recipes/chile-colorado.md#L8",
		},
		{
			name:         "relative file from repo on other tree not in cwd",
			workspaceCwd: reposPath + "/gitproj6_two-remotes",
			remoteName:   "origin",
			inputFile:    "../gitproj5_one-remote/recipes/chile-colorado.md",
			inputLine:    7,
			expect: "https://git.unstable.build/unstablebuild/gitproj5" +
				"/src/commit/b0c7a3e628f4bfb0cb60f2ee048b7e47017c57e7/recipes/chile-colorado.md#L8",
		},
		{
			name:         "remote name is honored",
			workspaceCwd: reposPath + "/gitproj6_two-remotes",
			remoteName:   "private-mirror",
			inputFile:    reposPath + "/gitproj6_two-remotes/recipes/guasacaca.md",
			inputLine:    7,
			expect: "https://git.unstable.build/unstablebuild/gitproj6-mirror" +
				"/src/commit/5367f818c4e7092a224ae0f32da1b7bab8843cc0/recipes/guasacaca.md#L8",
		},
		{
			name:      "unexistent remote name",
			mustError: true,
			// ERROR: remote url: process exit with non-zero status (exit
			// status 2) error: No such remote 'unexistent-remote-name'
			workspaceCwd: reposPath + "/gitproj6_two-remotes",
			remoteName:   "unexistent-remote-name",
			inputFile:    reposPath + "/gitproj6_two-remotes/recipes/chile-colorado.md",
			inputLine:    7,
		},
		{
			name:      "empty remote name",
			mustError: true,
			// ERROR: remote url: must pass remote name
			workspaceCwd: reposPath + "/gitproj6_two-remotes",
			remoteName:   "",
			inputFile:    reposPath + "/gitproj6_two-remotes/recipes/chile-colorado.md",
			inputLine:    7,
		},
		{
			name:         "github remotes",
			workspaceCwd: reposPath + "/gitproj6_two-remotes",
			remoteName:   "public-mirror",
			inputFile:    reposPath + "/gitproj6_two-remotes/recipes/guasacaca.md",
			inputLine:    7,
			expect: "https://github.com/unstablebuild/gitproj6-mirror" +
				"/src/commit/5367f818c4e7092a224ae0f32da1b7bab8843cc0/recipes/guasacaca.md#L8",
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			workspaceCwdURI, err := workspaceapi.ParseURI("file://" + tcase.workspaceCwd)
			require.NoError(t, err)

			git := setupGitService(t, workspaceCwdURI)
			res, err := remoteWebLink(git, tcase.remoteName, tcase.inputFile, tcase.inputLine)
			if tcase.mustError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tcase.expect, res)
		})
	}
}

func TestParseRemoteURL(t *testing.T) {
	tsuite := []struct {
		name      string
		input     string
		mustError bool
		expect    remoteURLParts
	}{
		{
			name:  "git http url",
			input: "https://git.unstable.build/unstablebuild/go-tui.git",
			expect: remoteURLParts{
				domain: "git.unstable.build",
				owner:  "unstablebuild",
				repo:   "go-tui",
			},
		},
		{
			name:  "git ssh url",
			input: "git@git.unstable.build:unstablebuild/go-tui.git",
			expect: remoteURLParts{
				domain: "git.unstable.build",
				owner:  "unstablebuild",
				repo:   "go-tui",
			},
		},
		{
			name:  "not ending with .git suffix",
			input: "git@git.unstable.build:unstablebuild/go-tui",
			expect: remoteURLParts{
				domain: "git.unstable.build",
				owner:  "unstablebuild",
				repo:   "go-tui",
			},
		},
		{
			name:      "owner part is namespaced",
			input:     "git@git.unstable.build:unstablebuild/namespace/go-tui.git",
			mustError: true,
		},
		{
			name:      "illegal url",
			input:     "git@git.unstable.build:unstablebuild/$%2go-tui.git",
			mustError: true,
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			res, err := parseRemoteURL(tcase.input)
			if tcase.mustError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tcase.expect, res)
		})
	}
}

func TestExpand(t *testing.T) {
	tsuite := []struct {
		name        string
		mustError   bool
		inputString string
		inputValues map[string]string
		expect      string
	}{
		{
			name:        "expand fills placeholders",
			inputString: ".:{a}-{b}:.",
			inputValues: map[string]string{"a": "1", "b": "2"},
			expect:      ".:1-2:.",
		},
		{
			name:      "expand leaving unfilled placeholders",
			mustError: true,
			// ERROR: template left with empty placeholders: .:1-{b}:.
			inputString: ".:{a}-{b}:.",
			inputValues: map[string]string{"a": "1"},
		},
		{
			name:        "expand giving extra placeholders",
			inputString: ".:{a}-{b}:.",
			inputValues: map[string]string{"a": "1", "b": "2", "c": "3"},
			expect:      ".:1-2:.",
		},
	}
	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			res, err := expand(tcase.inputString, tcase.inputValues)
			if tcase.mustError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tcase.expect, res)
		})
	}
}
