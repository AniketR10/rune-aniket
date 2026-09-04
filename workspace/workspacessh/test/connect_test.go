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

package workspacetest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/workspace/workspacessh"
)

// TestConnectSchemeEndToEnd exercises the full SSH workspace bootstrap
// against a real container running a `rune` binary on $PATH. This is
// the test that, when the bootstrap looked for an obsolete `six`
// binary, would have surfaced the bug the rest of the matrix missed
// because TestAuthDial short-circuits before whichCommand runs.
//
// It also proves connectScheme waits for the remote to be serving-ready
// before handing back a usable client: the runesvc stand-in is told to sleep
// before it starts serving (and before it emits the ServerReady sentinel), so
// the first RPC — which blocks on the background connect attempt via
// remoteScheme.state — must not resolve until the pre-serving delay has
// elapsed, yet must then succeed. Without readiness gating the client would be
// returned immediately and that first RPC would block indefinitely against a
// stdout that carries no gRPC server yet.
func TestConnectSchemeEndToEnd(t *testing.T) {
	SkipIfNoDocker(t)
	EnsureImage(t)

	const serveDelay = time.Second
	c := StartContainer(t, SSHDScenario{
		PublicKeyFile:     "/id_ed25519.pub",
		InstallRuneBinary: true,
		ServeDelay:        serveDelay.String(),
	})

	keyPath := PrivateKeyPath(t, "id_ed25519")

	// /tmp exists in the linuxserver/openssh-server image and is
	// writable by the test user; use it as the workspace root so
	// workspaceExists succeeds.
	uri, err := workspaceapi.ParseURI(
		fmt.Sprintf("ssh://test@%s/tmp", c.HostPort))
	require.NoError(t, err)

	cfg := config.MapConfig(map[string]any{
		"private_keys": []any{keyPath},
		"timeout":      "20s",
		"insecure":     true,
	})

	schemeFn := workspacessh.New(&errorUI{})
	scheme, err := schemeFn(context.Background(), cfg, uri)
	require.NoError(t, err)
	defer scheme.Close()

	// Stat the workspace path: the very first remote round trip. It blocks on
	// the background connect attempt (remoteScheme.state), which now completes
	// only once the remote is serving-ready. If the bootstrap looked for the
	// wrong binary or failed for any other reason this returns an error.
	start := time.Now()
	fi, err := scheme.Stat("/tmp")
	elapsed := time.Since(start)
	require.NoError(t, err, "Stat over the connected workspace should succeed")
	assert.NotNil(t, fi)

	// Allow scheduling slack below the injected delay: the point is that the
	// first RPC did not resolve early (which it would have without readiness
	// gating), not that it matches the delay exactly.
	assert.GreaterOrEqual(t, elapsed, serveDelay-300*time.Millisecond,
		"the first RPC must block until the remote is serving-ready; it "+
			"returned after %s but the remote delayed serving by %s",
		elapsed, serveDelay)
}

// TestConnectSchemeMultiKeyRedialBootstrap exercises the full
// scheme bootstrap (auth + whichCommand + Stat) on a server that
// caps MaxAuthTries to 1, with [wrong, right] keys configured.
// The pure-auth TestMaxAuthTriesOne stops the moment auth
// succeeds and never invokes the post-auth bootstrap, so it
// can't catch a regression where the per-key redial leaves the
// scheme in a state that breaks subsequent remote operations
// (e.g. dropped client config, cached failure, missing remote
// binary install in scenario scripts). This test guards that
// bigger surface.
func TestConnectSchemeMultiKeyRedialBootstrap(t *testing.T) {
	SkipIfNoDocker(t)
	EnsureImage(t)

	c := StartContainer(t, SSHDScenario{
		PublicKeyFile:     "/id_ed25519.pub",
		ExtraSSHDConfig:   "MaxAuthTries 1\n",
		InstallRuneBinary: true,
	})

	wrong := PrivateKeyPath(t, "id_ed25519_wrong")
	right := PrivateKeyPath(t, "id_ed25519")

	uri, err := workspaceapi.ParseURI(
		fmt.Sprintf("ssh://test@%s/tmp", c.HostPort))
	require.NoError(t, err)

	cfg := config.MapConfig(map[string]any{
		"private_keys": []any{wrong, right},
		"timeout":      "20s",
		"insecure":     true,
	})

	schemeFn := workspacessh.New(&errorUI{})
	scheme, err := schemeFn(context.Background(), cfg, uri)
	require.NoError(t, err)
	defer scheme.Close()

	// Stat triggers the connect path. Loop briefly to absorb
	// the maintainConnection goroutine's first publish.
	deadline := time.Now().Add(20 * time.Second)
	var fi any
	for time.Now().Before(deadline) {
		fi, err = scheme.Stat("/tmp")
		if err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	require.NoError(t, err,
		"per-key redial under MaxAuthTries=1 must complete the full "+
			"scheme bootstrap, not just the auth handshake; if this "+
			"fails it usually means the second key authenticates but "+
			"the bootstrap then can't find `rune` on the remote")
	assert.NotNil(t, fi)
}

// TestConnectSchemeUserLocalBin exercises the supported install
// location: install.sh symlinks the binary to ~/.local/bin/rune, a
// directory that is NOT on PATH in non-interactive SSH exec sessions
// (sshd runs a non-login shell, skipping the profile files where
// distributions add ~/.local/bin). The bootstrap must still find the
// binary via connectScheme's PATH injection — both in the `which`
// preflight and in the workspace-server launch itself.
func TestConnectSchemeUserLocalBin(t *testing.T) {
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

	schemeFn := workspacessh.New(&errorUI{})
	scheme, err := schemeFn(context.Background(), cfg, uri)
	require.NoError(t, err)
	defer scheme.Close()

	deadline := time.Now().Add(20 * time.Second)
	var fi any
	for time.Now().Before(deadline) {
		fi, err = scheme.Stat("/tmp")
		if err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	require.NoError(t, err,
		"the bootstrap must find `rune` installed only at "+
			"~/.local/bin/rune (install.sh's location) even though "+
			"that directory is not on the sshd session PATH")
	assert.NotNil(t, fi)
}
