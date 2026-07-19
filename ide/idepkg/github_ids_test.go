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

package idepkg

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"unstable.build/go-tui/ide/idepkg/idepkgtest"
)

const ghPkgID = "github.com/foo/bar"

func TestValidatePkgPath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in string
		ok bool
	}{
		// hierarchical github ids nest as real dirs and are accepted
		{"github.com/foo/bar", true},
		{"github.com/f-o_o./b.a-r_1", true},
		{"github.com/foo/bar/baz", true},
		{"go", true},
		{"a:b", true},
		// traversal-unsafe segments are rejected
		{"", false},
		{"github.com/../bar", false},
		{"..", false},
		{".", false},
		{"a/./b", false},
		{"a//b", false},
		{"/leading", false},
		{"trailing/", false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.ok, validatePkgPath(tc.in) == nil, "in=%q", tc.in)
	}
}

func ghPkgTarball(t *testing.T) []byte {
	return makeReqPkgTarball(t, map[string]string{
		"config.yaml": "extensions:\n  demo:\n" +
			"    path: $RUNE_DATADIR/lib/$RUNE_PKG_ID/main.py\n",
		"main.py": "print('hi')\n",
	})
}

func TestInstallGitHubID(t *testing.T) {
	t.Parallel()
	t.Run("install nests dirs and links lib", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: ghPkgID, Latest: "abc123def456"})
		versions := idepkgtest.MakeBundles(
			[]release.Bundle{{Package: ghPkgID, Version: "abc123def456"}})
		m, n, rm, temp := newTestManager(t, pkgs, versions)
		rm.SetTarball(ghPkgID, ghPkgTarball(t))

		err := m.InstallPackageVersion(
			context.Background(), ghPkgID, "abc123def456", repl.NopProgressWriter())
		require.NoError(t, err)
		n.RequireNoErrorNotification()

		version, ok := m.PackageVersionInUse(ghPkgID)
		require.True(t, ok)
		assert.Equal(t, release.Version("abc123def456"), version)

		libDir := filepath.Join(temp, "lib", "github.com", "foo", "bar")
		target, err := os.Readlink(libDir)
		require.NoError(t, err)
		assert.Equal(t,
			filepath.Join(temp, "pkg", "github.com", "foo", "bar", "abc123def456"),
			target)

		it, err := m.LibDir(context.Background(), ghPkgID)
		require.NoError(t, err)
		files, err := iterator.ToSlice(context.Background(), it)
		require.NoError(t, err)
		var names []string
		for _, f := range files {
			names = append(names, filepath.Base(f))
		}
		assert.ElementsMatch(t, []string{"config.yaml", "main.py"}, names)
	})
	t.Run("requirements of a github package install first", func(t *testing.T) {
		t.Parallel()
		const depID = "python"
		pkgs := idepkgtest.MakePackages(
			release.Package{Name: ghPkgID, Latest: "abc123def456"},
			release.Package{Name: depID, Latest: "1"},
		)
		versions := idepkgtest.MakeBundles(
			[]release.Bundle{{Package: ghPkgID, Version: "abc123def456"}},
			[]release.Bundle{{Package: depID, Version: "1"}},
		)
		m, n, rm, _ := newTestManager(t, pkgs, versions)
		rm.SetTarball(ghPkgID, makeReqPkgTarball(t, map[string]string{
			"config.yaml": "requirements:\n  - " + depID + "\n" +
				"extensions:\n  demo:\n" +
				"    path: $RUNE_DATADIR/lib/$RUNE_PKG_ID/main.py\n",
			"main.py": "print('hi')\n",
		}))
		rm.SetTarball(depID, makeReqPkgTarball(t, map[string]string{
			"lib/python.txt": "python\n",
		}))

		err := m.InstallPackageVersion(
			context.Background(), ghPkgID, "abc123def456", repl.NopProgressWriter())
		require.NoError(t, err)
		n.RequireNoErrorNotification()

		_, ok := m.PackageVersionInUse(depID)
		assert.True(t, ok, "requirement must be installed")
		_, ok = m.PackageVersionInUse(ghPkgID)
		assert.True(t, ok, "github package must be installed")
	})
	t.Run("delete package cleans up nested dirs", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(release.Package{Name: ghPkgID, Latest: "abc123def456"})
		versions := idepkgtest.MakeBundles(
			[]release.Bundle{{Package: ghPkgID, Version: "abc123def456"}})
		m, n, rm, temp := newTestManager(t, pkgs, versions)
		rm.SetTarball(ghPkgID, ghPkgTarball(t))

		err := m.InstallPackageVersion(
			context.Background(), ghPkgID, "abc123def456", repl.NopProgressWriter())
		require.NoError(t, err)
		n.RequireNoErrorNotification()

		require.NoError(t, m.DeletePackage(context.Background(), ghPkgID))

		_, ok := m.PackageVersionInUse(ghPkgID)
		assert.False(t, ok)
		_, err = os.Stat(filepath.Join(temp, "pkg", "github.com", "foo", "bar"))
		assert.True(t, os.IsNotExist(err), "pkg dir must be removed")
		_, err = os.Lstat(filepath.Join(temp, "lib", "github.com", "foo", "bar"))
		assert.True(t, os.IsNotExist(err), "lib symlink must be removed")

		assert.ErrorIs(t,
			m.DeletePackage(context.Background(), ghPkgID), ErrNotInstalled)
	})
}

func TestReconcileCleansNestedStaging(t *testing.T) {
	t.Parallel()
	pkgs := idepkgtest.MakePackages()
	versions := idepkgtest.MakeBundles()
	m, _, _, temp := newTestManager(t, pkgs, versions)

	// Simulate a crash mid-install: an incomplete storage entry with a
	// leftover staging dir nested too deep for the depth-1 sweep.
	ctx := context.Background()
	key := m.makeDownloadKey(ghPkgID, "abc123def456")
	require.NoError(t, m.storage.Create(ctx, key,
		newPkgVersionValue(ghPkgID, "abc123def456")))
	stagingDir := makeStagingDirname(temp, ghPkgID, "abc123def456")
	require.NoError(t, os.MkdirAll(stagingDir, 0o777))

	require.NoError(t, m.Reconcile(ctx))

	_, err := os.Stat(stagingDir)
	assert.True(t, os.IsNotExist(err), "staging dir must be removed")
	var val pkgVersionValue
	assert.Error(t, m.storage.Get(ctx, key, &val),
		"incomplete storage entry must be removed")
}
