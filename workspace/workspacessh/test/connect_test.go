// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.

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
	"unstable.build/go-tui/workspace/workspacessh"
)

// TestConnectSchemeEndToEnd exercises the full SSH workspace bootstrap
// against a real container running a `rune` binary on $PATH. This is
// the test that, when the bootstrap looked for an obsolete `six`
// binary, would have surfaced the bug the rest of the matrix missed
// because TestAuthDial short-circuits before whichCommand runs.
func TestConnectSchemeEndToEnd(t *testing.T) {
	SkipIfNoDocker(t)
	EnsureImage(t)

	c := StartContainer(t, SSHDScenario{
		PublicKeyFile:     "/id_ed25519.pub",
		InstallRuneBinary: true,
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

	// Stat the workspace path: the very first remote round trip. If
	// the bootstrap was looking for the wrong binary or failed for any
	// other reason this returns an error.
	deadline := time.Now().Add(20 * time.Second)
	var fi any
	for time.Now().Before(deadline) {
		fi, err = scheme.Stat("/tmp")
		if err == nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	require.NoError(t, err, "Stat over the connected workspace should succeed")
	assert.NotNil(t, fi)
}
