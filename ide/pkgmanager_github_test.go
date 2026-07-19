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

package ide

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-git/go-billy/v6/osfs"
	git "github.com/go-git/go-git/v6"
	backendhttp "github.com/go-git/go-git/v6/backend/http"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/plumbing/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"unstable.build/go-tui/ide/idepkg"
	"unstable.build/go-tui/ide/idepkg/idepkgtest"
	"unstable.build/go-tui/ide/pkgshell"
)

// gitFixtureCommit writes files into a fixture repo worktree and
// commits them, returning the commit SHA.
func gitFixtureCommit(t *testing.T, dir string, files map[string]string) string {
	t.Helper()
	repo, err := git.PlainOpen(dir)
	if err != nil {
		repo, err = git.PlainInit(dir, false)
		require.NoError(t, err)
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o777))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	}
	wt, err := repo.Worktree()
	require.NoError(t, err)
	_, err = wt.Add(".")
	require.NoError(t, err)
	sha, err := wt.Commit("fixture", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "fixture",
			Email: "fixture@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)
	return sha.String()
}

// serveGitFixtures serves every repo under base over git smart-HTTP and
// returns a mapper from git package ID to its fixture remote URL,
// suitable for a handler's gitRemoteURL field.
func serveGitFixtures(t *testing.T, base string) func(pkgID string) string {
	t.Helper()
	srv := httptest.NewServer(backendhttp.NewBackend(
		transport.NewFilesystemLoader(osfs.New(base), false)))
	t.Cleanup(srv.Close)
	return func(pkgID string) string { return srv.URL + "/" + pkgID }
}

func TestPkgManagerGitHubInstallFacade(t *testing.T) {
	const ghID = "github.com/owner/repo"
	base := t.TempDir()
	repoDir := filepath.Join(base, "github.com", "owner", "repo")
	sha := gitFixtureCommit(t, repoDir, map[string]string{
		"config.yaml": "requirements:\n  - six\n" +
			"extensions:\n  demo:\n" +
			"    path: $RUNE_DATADIR/lib/$RUNE_PKG_ID/main.py\n",
		"main.py": "print('hi')\n",
	})
	ghURL := serveGitFixtures(t, base)

	pkgs := idepkgtest.MakePackages(release.Package{Name: "six", Latest: "2"})
	bundles := idepkgtest.MakeBundles(
		[]release.Bundle{{Package: "six", Version: "2"}})
	rm := idepkgtest.NewReleaseManager(pkgs, bundles)
	m := newTestWorkspaceManagerHandlerForPkgManager(t, rm, false, 0, ghURL)
	defer m.Close()

	h := pkgshell.New(pkgshell.Config{
		Manager:       m.pkgmanager.pkg,
		UpdateChecker: m.pkgmanager.uc,
	})
	ctx := context.Background()

	// install resolves HEAD as the latest version, clones, and installs
	// the declared requirement from the official distribution first.
	_, err := h.HandleCommand(ctx, repl.Command{
		Name: pkgshell.CommandName,
		Args: []string{"install", ghID},
	}, repl.NopProgressWriter())
	require.NoError(t, err)

	version, ok := m.pkgmanager.pkg.PackageVersionInUse(ghID)
	require.True(t, ok)
	assert.Equal(t, release.Version(sha[:12]), version)
	_, ok = m.pkgmanager.pkg.PackageVersionInUse("six")
	assert.True(t, ok, "requirement must be installed from the official distribution")

	it, err := m.pkgmanager.LibDir(ctx, ghID)
	require.NoError(t, err)
	var names []string
	for {
		f, ok := it.Next(ctx)
		if !ok {
			break
		}
		names = append(names, filepath.Base(f))
	}
	require.NoError(t, it.Err())
	require.NoError(t, it.Close())
	assert.ElementsMatch(t, []string{"config.yaml", "main.py"}, names)

	// a new commit surfaces as an available update
	sha2 := gitFixtureCommit(t, repoDir, map[string]string{
		"main.py": "print('v2')\n",
	})
	updates, err := m.pkgmanager.uc.CheckForUpdates(ctx)
	require.NoError(t, err)
	require.Len(t, updates, 1)
	assert.Equal(t, idepkg.Update{
		Package: ghID,
		Current: release.Version(sha[:12]),
		Latest:  release.Version(sha2[:12]),
	}, updates[0])

	// remove cleans up through the shell
	_, err = h.HandleCommand(ctx, repl.Command{
		Name: pkgshell.CommandName,
		Args: []string{"remove", ghID},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	_, ok = m.pkgmanager.pkg.PackageVersionInUse(ghID)
	assert.False(t, ok)
}
