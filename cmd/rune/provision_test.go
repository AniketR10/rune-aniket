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

package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"unstable.build/go-tui/ide/idepkg"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/workspacessh"
)

// setFlagForTest temporarily points a *string flag at value and restores it.
func setFlagForTest(t *testing.T, target *string, value string) {
	t.Helper()
	old := *target
	*target = value
	t.Cleanup(func() { *target = old })
}

func newTestFileScheme(t *testing.T, root string) (workspace.Workspace, workspaceapi.URI) {
	t.Helper()
	uri, err := workspaceapi.CurrentUserHostURI(root)
	require.NoError(t, err)
	scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), uri)
	require.NoError(t, err)
	t.Cleanup(func() { _ = scheme.Close() })
	cwd := workspace.NewSchemeWorkspace(uri, scheme, func(fn func()) bool { fn(); return true })
	return cwd, uri
}

// TestLoadRemoteConfigAppliesGUIEnvAfterInstall asserts the load-after-install
// ordering on the -x path: once a package install has merged a gui.env block
// into the remote ~/.rune config, loading that config applies the vars to the
// process environment so extension-spawned tools inherit them.
func TestLoadRemoteConfigAppliesGUIEnvAfterInstall(t *testing.T) {
	const (
		envKey = "RUNE_TEST_REMOTE_GOROOT"
		envVal = "/remote/go/root"
	)
	require.NoError(t, os.Unsetenv(envKey))
	t.Cleanup(func() { _ = os.Unsetenv(envKey) })

	dataDir := t.TempDir()
	configPath := filepath.Join(dataDir, "config.yaml")
	// The remote config as it exists AFTER a package install merged its
	// gui.env block.
	merged := "editor:\n  mode: modal\ngui:\n  env:\n    " + envKey + ": " + envVal + "\n"
	require.NoError(t, os.WriteFile(configPath, []byte(merged), 0o644))

	setFlagForTest(t, flagConfigPath, configPath)
	setFlagForTest(t, flagDataPath, dataDir)

	cwd, uri := newTestFileScheme(t, t.TempDir())
	cfg := loadRemoteConfigAndApplyEnv(cwd, uri)
	require.NotNil(t, cfg)

	assert.Equal(t, envVal, os.Getenv(envKey),
		"gui.env from the remote config must be applied to the process env")

	// ~/.rune/bin must be on PATH so package executables resolve.
	assert.Contains(t, os.Getenv("PATH"), filepath.Join(dataDir, "bin"),
		"~/.rune/bin must be prepended to PATH")
}

// TestWorkspaceOverlayWinsOverHomeConfig asserts the workspace-root
// .rune/config.yaml overlay is layered on top of the remote-home config.
func TestWorkspaceOverlayWinsOverHomeConfig(t *testing.T) {
	const envKey = "RUNE_TEST_OVERLAY_VAR"
	require.NoError(t, os.Unsetenv(envKey))
	t.Cleanup(func() { _ = os.Unsetenv(envKey) })

	dataDir := t.TempDir()
	configPath := filepath.Join(dataDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath,
		[]byte("editor:\n  mode: modal\ngui:\n  env:\n    "+envKey+": home\n"), 0o644))

	setFlagForTest(t, flagConfigPath, configPath)
	setFlagForTest(t, flagDataPath, dataDir)

	wsRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, ".rune"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(wsRoot, workspaceConfigFilename),
		[]byte("gui:\n  env:\n    "+envKey+": workspace\n"), 0o644))

	cwd, uri := newTestFileScheme(t, wsRoot)
	loadRemoteConfigAndApplyEnv(cwd, uri)

	assert.Equal(t, "workspace", os.Getenv(envKey),
		"workspace-root .rune/config.yaml must override the home config")
}

// TestInstallRemotePackagesEmptyManifestIsNoop asserts an empty --install
// manifest performs no install work (and does not panic).
func TestInstallRemotePackagesEmptyManifestIsNoop(t *testing.T) {
	setFlagForTest(t, flagWorkspaceServerInstall, "")
	setFlagForTest(t, flagDataPath, t.TempDir())
	cwd, _ := newTestFileScheme(t, t.TempDir())
	// Should return immediately without touching storage or the network.
	installRemotePackages(cwd)
}

// TestInstallRemotePackagesEmptyManifestEmitsNothing asserts that with no
// packages to install, nothing is written to the progress sink.
func TestInstallRemotePackagesEmptyManifestEmitsNothing(t *testing.T) {
	setFlagForTest(t, flagWorkspaceServerInstall, "")
	setFlagForTest(t, flagDataPath, t.TempDir())
	cwd, _ := newTestFileScheme(t, t.TempDir())

	var sink strings.Builder
	installRemotePackagesTo(cwd, &sink)
	assert.Empty(t, sink.String(), "empty manifest must emit no progress lines")
}

// TestInstallRemotePackagesEmitsOrderedProgress asserts the ordered JSON-Lines
// progress stream. The release endpoint is pointed at an unreachable address so
// each install fails offline, exercising the installing → failed sequence per
// package followed by a final done line — without any network access.
func TestInstallRemotePackagesEmitsOrderedProgress(t *testing.T) {
	setFlagForTest(t, flagWorkspaceServerInstall, "pkg-a@1.0.0,pkg-b@2.0.0")
	setFlagForTest(t, flagDataPath, t.TempDir())
	setFlagForTest(t, flagConfigPath, filepath.Join(t.TempDir(), "config.yaml"))
	// 127.0.0.1:0 is not a listening endpoint, so release downloads fail fast.
	setFlagForTest(t, flagHTTPAddress, "http://127.0.0.1:0")
	cwd, _ := newTestFileScheme(t, t.TempDir())

	var sink strings.Builder
	installRemotePackagesTo(cwd, &sink)

	var got []workspacessh.ProvisionProgress
	scanner := bufio.NewScanner(strings.NewReader(sink.String()))
	for scanner.Scan() {
		p, ok := workspacessh.ParseProvisionProgressLine(scanner.Bytes())
		require.True(t, ok, "every emitted line must parse as progress: %q", scanner.Text())
		got = append(got, p)
	}
	require.NoError(t, scanner.Err())

	// Failed lines must carry the underlying error text so the notification
	// can explain the failure. Assert it is present, then normalize it out to
	// compare the ordered structure against a fixed expectation.
	for i := range got {
		if got[i].Phase == workspacessh.ProvisionPhaseFailed {
			assert.NotEmpty(t, got[i].Detail,
				"failed line %d must carry the install error detail", i)
			got[i].Detail = ""
		}
	}

	want := []workspacessh.ProvisionProgress{
		{Rune: "provision", Index: 1, Total: 2, Package: "pkg-a", Version: "1.0.0", Phase: workspacessh.ProvisionPhaseInstalling},
		{Rune: "provision", Index: 1, Total: 2, Package: "pkg-a", Version: "1.0.0", Phase: workspacessh.ProvisionPhaseFailed},
		{Rune: "provision", Index: 2, Total: 2, Package: "pkg-b", Version: "2.0.0", Phase: workspacessh.ProvisionPhaseInstalling},
		{Rune: "provision", Index: 2, Total: 2, Package: "pkg-b", Version: "2.0.0", Phase: workspacessh.ProvisionPhaseFailed},
		{Rune: "provision", Index: 2, Total: 2, Phase: workspacessh.ProvisionPhaseDone},
	}
	assert.Equal(t, want, got)
}

// fakeInstaller stands in for the provisioning manager so the version-fallback
// logic can be tested without network or storage.
type fakeInstaller struct {
	// installErr[version] is returned by InstallPackageVersion for that version.
	installErr map[string]error
	latest     map[string]release.Version
	latestErr  map[string]error

	installed []string // versions passed to InstallPackageVersion, in order
	used      []string // versions passed to UsePackageVersion, in order
}

func (f *fakeInstaller) InstallPackageVersion(
	_ context.Context, _ string, version release.Version, _ repl.ProgressWriter,
) error {
	f.installed = append(f.installed, string(version))
	return f.installErr[string(version)]
}

func (f *fakeInstaller) UsePackageVersion(
	_ context.Context, _ string, version release.Version,
) error {
	f.used = append(f.used, string(version))
	return nil
}

func (f *fakeInstaller) LatestVersion(
	_ context.Context, id string,
) (release.Version, error) {
	if err := f.latestErr[id]; err != nil {
		return "", err
	}
	return f.latest[id], nil
}

// TestInstallOnePackageFallsBackToLatest asserts that when the pinned version is
// not published for the remote platform (ErrVersionNotFound), the install
// degrades to the latest available version and reports it via progress.
func TestInstallOnePackageFallsBackToLatest(t *testing.T) {
	inst := &fakeInstaller{
		installErr: map[string]error{
			"v1.2.3": fmt.Errorf("nope: %w", idepkg.ErrVersionNotFound),
			"v9.9.9": nil,
		},
		latest: map[string]release.Version{"go": "v9.9.9"},
	}
	var sink strings.Builder
	ok := installOnePackage(context.Background(), inst, &sink,
		idepkg.ProvisionEntry{ID: "go", Version: "v1.2.3"}, 1, 1)
	require.True(t, ok, "fallback install must succeed")

	assert.Equal(t, []string{"v1.2.3", "v9.9.9"}, inst.installed,
		"must try the pinned version, then the latest")
	assert.Equal(t, []string{"v9.9.9"}, inst.used,
		"must activate the fallback version")

	var got []workspacessh.ProvisionProgress
	scanner := bufio.NewScanner(strings.NewReader(sink.String()))
	for scanner.Scan() {
		p, ok := workspacessh.ParseProvisionProgressLine(scanner.Bytes())
		require.True(t, ok)
		got = append(got, p)
	}
	// installing(pinned) then activating(fallback) — the activating line must
	// reflect the version actually installed, not the pinned one.
	require.Len(t, got, 2)
	assert.Equal(t, workspacessh.ProvisionPhaseInstalling, got[0].Phase)
	assert.Equal(t, workspacessh.ProvisionPhaseActivating, got[1].Phase)
	assert.Equal(t, "v9.9.9", got[1].Version,
		"activating progress must show the fallback version")
}

// TestInstallOnePackagePinnedSucceeds asserts the happy path: when the pinned
// version installs, no fallback is attempted.
func TestInstallOnePackagePinnedSucceeds(t *testing.T) {
	inst := &fakeInstaller{installErr: map[string]error{"v1.2.3": nil}}
	var sink strings.Builder
	ok := installOnePackage(context.Background(), inst, &sink,
		idepkg.ProvisionEntry{ID: "go", Version: "v1.2.3"}, 1, 1)
	require.True(t, ok)
	assert.Equal(t, []string{"v1.2.3"}, inst.installed)
	assert.Equal(t, []string{"v1.2.3"}, inst.used)
}

// TestInstallOnePackageFailsWhenNoFallback asserts that a non-version-not-found
// error is not retried, and a failed progress line is emitted.
func TestInstallOnePackageFailsWhenNoFallback(t *testing.T) {
	inst := &fakeInstaller{
		installErr: map[string]error{"v1.2.3": errors.New("network down")},
	}
	var sink strings.Builder
	ok := installOnePackage(context.Background(), inst, &sink,
		idepkg.ProvisionEntry{ID: "go", Version: "v1.2.3"}, 1, 1)
	require.False(t, ok)
	assert.Equal(t, []string{"v1.2.3"}, inst.installed, "must not retry on a non-404 error")
	assert.Contains(t, sink.String(), "failed")
}

// TestInstallOnePackageFallbackAlsoMissing asserts that if both the pinned and
// the latest versions are unavailable, the package fails cleanly.
func TestInstallOnePackageFallbackAlsoMissing(t *testing.T) {
	inst := &fakeInstaller{
		installErr: map[string]error{
			"v1.2.3": fmt.Errorf("x: %w", idepkg.ErrVersionNotFound),
		},
		latestErr: map[string]error{"go": errors.New("no releases")},
	}
	var sink strings.Builder
	ok := installOnePackage(context.Background(), inst, &sink,
		idepkg.ProvisionEntry{ID: "go", Version: "v1.2.3"}, 1, 1)
	require.False(t, ok)
	assert.Contains(t, sink.String(), "failed")
}
