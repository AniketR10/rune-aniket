// Copyright (C) 2017-2026 Unstable Build, LLC
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

//go:build e2e

package workspacetest

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/internal/workspace/workspacessh"
)

// TestConnectSchemeProvisionAppliesGUIEnv is the end-to-end proof that the
// remote `rune -x` server applies its gui.env block before it starts serving.
// The local side passes a non-empty provision manifest (via
// WithProvisionManifest), which threads `--install <manifest>` into the remote
// launch. The remote (runesvc stand-in) then loads ~/.rune/config.yaml — seeded
// here with a sentinel gui.env var, as a package install would have merged —
// and applies it to its process env. A command run over the connected
// workspace inherits that env, so observing the sentinel proves the ordering:
// provisioning ran, config loaded, gui.env applied, all before serving.
func TestConnectSchemeProvisionAppliesGUIEnv(t *testing.T) {
	SkipIfNoDocker(t)
	EnsureImage(t)

	const sentinel = "RUNE_PROVISION_OK"
	c := StartContainer(t, SSHDScenario{
		PublicKeyFile:     "/id_ed25519.pub",
		InstallRuneBinary: true,
		RemoteHomeConfig:  "gui:\n  env:\n    " + sentinel + ": \"1\"\n",
	})

	keyPath := PrivateKeyPath(t, "id_ed25519")

	uri, err := workspaceapi.ParseURI(
		fmt.Sprintf("ssh://test@%s/tmp", c.HostPort))
	require.NoError(t, err)

	cfg := config.MapConfig(map[string]any{
		"private_keys": []any{keyPath},
		"timeout":      "20s",
		"insecure":     true,
	})

	// A non-empty manifest makes connectScheme append --install, which
	// switches the runesvc stand-in into its provisioning path.
	//
	// A two-entry manifest also exercises the ordered progress stream: the
	// runesvc stand-in emits installing/activating lines per package plus a
	// final done line on stderr, and the local scanner forwards each to
	// ui.Notify. We record those notifications and assert the order below.
	ui := &notifyRecordingUI{}
	schemeFn := workspacessh.New(ui,
		workspacessh.WithProvisionManifest(func() string {
			return "pkg-a@1.0.0,pkg-b@2.0.0"
		}))
	scheme, err := schemeFn(context.Background(), cfg, uri)
	require.NoError(t, err)
	defer scheme.Close()

	// Run a command over the connected workspace and read the sentinel from
	// the server's environment. The child inherits the -x server's env, so a
	// non-empty value proves gui.env was applied before serving.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var stdout bytes.Buffer
	ch := make(chan error, 1)
	cmd := workspaceapi.Cmd{
		Path:    "sh",
		Args:    []string{"-c", "printf %s \"$" + sentinel + "\""},
		Stdout:  &stdout,
		Watcher: workspaceapi.ChanProcessWatcher(ch),
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		_, err = scheme.StartCommand(ctx, cmd)
		if err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	require.NoError(t, err, "StartCommand over connected workspace should succeed")

	select {
	case err := <-ch:
		require.NoError(t, err, "remote command should exit cleanly; stdout=%q",
			stdout.String())
	case <-ctx.Done():
		t.Fatalf("timed out waiting for remote command; stdout=%q", stdout.String())
	}

	assert.Equal(t, "1", stdout.String(),
		"the remote rune -x server must apply its gui.env sentinel before "+
			"serving, so a spawned command inherits %s=1", sentinel)

	// The remote emitted its provisioning progress on stderr before serving;
	// the local scanner drives one live progress notification whose message
	// advances through each phase and closes only when the ServerReady line
	// arrives. The scanner runs on its own goroutine, so allow a brief settle
	// window until the final "Workspace ready" update lands.
	var updates []progressUpdate
	deadline = time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		updates = ui.progress()
		if len(updates) >= 7 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	msgs := make([]string, len(updates))
	for i, u := range updates {
		msgs[i] = u.message
	}
	assert.Equal(t, []string{
		"Installing toolchain (1/2): pkg-a@1.0.0",
		"Activated pkg-a@1.0.0",
		"Installing toolchain (2/2): pkg-b@2.0.0",
		"Activated pkg-b@2.0.0",
		"Installed 2/2 toolchain packages",
		"Finalizing workspace…",
		"Workspace ready",
	}, msgs, "provisioning progress must advance the notification in order and "+
		"close only at serving-ready")

	// Exactly one Notify opened the progress bar (info level); the rest of the
	// stream flows through UpdateNotificationProgress on the same id.
	assert.Equal(t, []string{"Installing toolchain (1/2): pkg-a@1.0.0"},
		ui.messages(), "a single Notify opens the progress notification")
	require.NotEmpty(t, updates)
	// The bar stays open through the finalize phase (Installed/Finalizing hold
	// below total) and completes only when serving-ready closes it.
	installedIdx := 4
	assert.Less(t, updates[installedIdx].progress, updates[installedIdx].total,
		"the done line must not close the bar")
	assert.Less(t, updates[installedIdx+1].progress, updates[installedIdx+1].total,
		"the finalizing line must not close the bar")
	last := updates[len(updates)-1]
	assert.Equal(t, last.total, last.progress,
		"only serving-ready completes the progress bar")
}

// notifyRecordingUI records Notify messages and progress updates so the e2e
// test can assert the ordered provisioning progress reached the browser
// surface. Prompts are never expected on the key-auth path and fail loudly.
type notifyRecordingUI struct {
	mu         sync.Mutex
	notis      []string
	progresses []progressUpdate
}

// progressUpdate records one UpdateNotificationProgress call.
type progressUpdate struct {
	id       string
	message  string
	progress int
	total    int
}

func (u *notifyRecordingUI) PromptSecret(context.Context, string) (string, error) {
	return "", fmt.Errorf("unexpected prompt: secret")
}

func (u *notifyRecordingUI) PromptText(context.Context, string, string) (string, error) {
	return "", fmt.Errorf("unexpected prompt: text")
}

func (u *notifyRecordingUI) PromptChoice(context.Context, string, []string) (int, error) {
	return -1, fmt.Errorf("unexpected prompt: choice")
}

func (u *notifyRecordingUI) Notify(_ workspacessh.NotificationLevel, msg string) string {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.notis = append(u.notis, msg)
	return "noti-" + msg
}

func (u *notifyRecordingUI) UpdateNotificationProgress(id, message string, progress, total int) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.progresses = append(u.progresses, progressUpdate{
		id: id, message: message, progress: progress, total: total,
	})
}

func (u *notifyRecordingUI) messages() []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]string(nil), u.notis...)
}

func (u *notifyRecordingUI) progress() []progressUpdate {
	u.mu.Lock()
	defer u.mu.Unlock()
	return append([]progressUpdate(nil), u.progresses...)
}

// TestConnectSchemeRunProvisionedPackageExecutable is the end-to-end proof that
// once a workspace is open, a package installed during provisioning can be run
// by bare name through the workspace executor. The runesvc stand-in stages an
// executable per manifest entry into ~/.rune/bin and prepends that directory to
// PATH before serving — exactly as the real -x server does via setupRuneBinPATH
// — so a command spawned over the workspace RPC resolves the provisioned tool
// from PATH and runs it.
func TestConnectSchemeRunProvisionedPackageExecutable(t *testing.T) {
	SkipIfNoDocker(t)
	EnsureImage(t)

	c := StartContainer(t, SSHDScenario{
		PublicKeyFile:     "/id_ed25519.pub",
		InstallRuneBinary: true,
	})

	keyPath := PrivateKeyPath(t, "id_ed25519")

	uri, err := workspaceapi.ParseURI(
		fmt.Sprintf("ssh://test@%s/tmp", c.HostPort))
	require.NoError(t, err)

	cfg := config.MapConfig(map[string]any{
		"private_keys": []any{keyPath},
		"timeout":      "20s",
		"insecure":     true,
	})

	const pkg = "rune-tool"
	schemeFn := workspacessh.New(&notifyRecordingUI{},
		workspacessh.WithProvisionManifest(func() string {
			return pkg + "@1.2.3"
		}))
	scheme, err := schemeFn(context.Background(), cfg, uri)
	require.NoError(t, err)
	defer scheme.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Run the provisioned tool by bare name (no path): resolving it proves
	// ~/.rune/bin, where provisioning placed the executable, is on the served
	// process's PATH and is inherited by the spawned command.
	var stdout bytes.Buffer
	ch := make(chan error, 1)
	cmd := workspaceapi.Cmd{
		Path:    pkg,
		Stdout:  &stdout,
		Watcher: workspaceapi.ChanProcessWatcher(ch),
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		_, err = scheme.StartCommand(ctx, cmd)
		if err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	require.NoError(t, err,
		"the provisioned package executable must be runnable by bare name "+
			"over the connected workspace")

	select {
	case err := <-ch:
		require.NoError(t, err, "provisioned tool should exit cleanly; stdout=%q",
			stdout.String())
	case <-ctx.Done():
		t.Fatalf("timed out running provisioned tool; stdout=%q", stdout.String())
	}

	assert.Equal(t, pkg+" ok 1.2.3\n", stdout.String(),
		"running the provisioned package executable must produce its output")
}
