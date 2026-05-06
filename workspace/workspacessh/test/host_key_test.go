// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.

package workspacetest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/workspace/workspacessh"
)

// readTestdataFile reads testdata/<name> with an absolute path so
// the calling test does not rely on the cwd. Used by host-key tests
// to embed a different-but-valid ed25519 public key in known_hosts.
func readTestdataFile(t *testing.T, name string) []byte {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("read %s: %v", abs, err)
	}
	return b
}

// TestHostKeyMatchingKnownHosts pins the std remote against a real
// container with Insecure=false and a known_hosts file that records
// the container's actual ed25519 host key. The dial must succeed
// and produce no auth prompts.
//
// Coverage gap addressed: TestAuthMatrix always uses Insecure=true,
// so the host-key callback path is never exercised end-to-end.
func TestHostKeyMatchingKnownHosts(t *testing.T) {
	SkipIfNoDocker(t)
	EnsureImage(t)
	t.Parallel()

	c := StartContainer(t, SSHDScenario{PublicKeyFile: "/id_ed25519.pub"})
	khPath := WriteKnownHosts(t, c.HostPort, ContainerHostPubKeys(t, c.ID)...)

	uri, err := workspaceapi.ParseURI("ssh://test@" + c.HostPort + "/")
	require.NoError(t, err)

	keyPath := PrivateKeyPath(t, "id_ed25519")
	ui := &recordingUI{}
	err = workspacessh.TestAuthDial(context.Background(), ui, uri,
		workspacessh.AuthDialOptions{
			PrivateKeys:    []string{keyPath},
			Insecure:       false,
			KnownHostsPath: khPath,
			Timeout:        20 * time.Second,
		})
	require.NoError(t, err,
		"dial against a server whose host key is in known_hosts must "+
			"succeed even with Insecure=false; if this fails the host "+
			"key callback is rejecting otherwise-valid keys")
	assert.Empty(t, ui.prompts,
		"matching-known_hosts dial must not prompt the user; saw %v", ui.prompts)
}

// TestHostKeyMismatchSurfacesErrHostKeyMismatch dials with
// Insecure=false and a known_hosts file whose recorded key does not
// match what the server presents. The dial must fail with a typed
// ErrHostKeyMismatch — never a password prompt — because that
// scenario typically indicates a man-in-the-middle attack.
func TestHostKeyMismatchSurfacesErrHostKeyMismatch(t *testing.T) {
	SkipIfNoDocker(t)
	EnsureImage(t)
	t.Parallel()

	c := StartContainer(t, SSHDScenario{PublicKeyFile: "/id_ed25519.pub"})

	// Pin a *different* but well-formed ed25519 public key so that
	// knownhosts.New parses the file and the dial reaches the actual
	// fingerprint comparison. We reuse id_ed25519.pub from testdata
	// (a checked-in client key) — it is a syntactically valid
	// ed25519 public key that happens not to match what the server
	// presents.
	wrong := readTestdataFile(t, "id_ed25519.pub")
	khPath := WriteKnownHosts(t, c.HostPort, wrong)

	uri, err := workspaceapi.ParseURI("ssh://test@" + c.HostPort + "/")
	require.NoError(t, err)

	keyPath := PrivateKeyPath(t, "id_ed25519")
	ui := &recordingUI{}
	err = workspacessh.TestAuthDial(context.Background(), ui, uri,
		workspacessh.AuthDialOptions{
			PrivateKeys:    []string{keyPath},
			Insecure:       false,
			KnownHostsPath: khPath,
			Timeout:        20 * time.Second,
		})
	require.Error(t, err)
	assert.True(t, errors.Is(err, workspacessh.ErrHostKeyMismatch),
		"mismatched known_hosts must surface ErrHostKeyMismatch so "+
			"isRetryableConnectError stops the reconnect loop and we "+
			"never silently roll past a possible MITM; got %v", err)
	assert.Empty(t, ui.prompts,
		"host-key mismatch must NOT prompt the user — that would let "+
			"an attacker harvest credentials; saw %v", ui.prompts)
}

// TestHostKeyChangedBetweenDials reproduces the "host has been
// rebuilt and got a new key" scenario. We dial once with a
// known_hosts file pinning the original ed25519 host key, succeed,
// then regenerate the key on the container and dial again with the
// SAME known_hosts file. The second dial must fail with
// ErrHostKeyMismatch.
func TestHostKeyChangedBetweenDials(t *testing.T) {
	SkipIfNoDocker(t)
	EnsureImage(t)
	t.Parallel()

	c := StartContainer(t, SSHDScenario{PublicKeyFile: "/id_ed25519.pub"})
	khPath := WriteKnownHosts(t, c.HostPort, ContainerHostPubKeys(t, c.ID)...)

	uri, err := workspaceapi.ParseURI("ssh://test@" + c.HostPort + "/")
	require.NoError(t, err)

	keyPath := PrivateKeyPath(t, "id_ed25519")
	opts := workspacessh.AuthDialOptions{
		PrivateKeys:    []string{keyPath},
		Insecure:       false,
		KnownHostsPath: khPath,
		Timeout:        20 * time.Second,
	}

	// First dial: original host key. Must succeed.
	require.NoError(t, workspacessh.TestAuthDial(
		context.Background(), &recordingUI{}, uri, opts),
		"baseline dial against the original host key must succeed; "+
			"the rotation half of this test is meaningless if the "+
			"first dial already fails")

	// Rotate every host key on the server. known_hosts on disk is
	// untouched, so the next dial must observe the mismatch
	// regardless of which algorithm HostKeyAlgorithms negotiates.
	RegenerateContainerHostKeys(t, c.ID)
	if err := waitForSSH(c.HostPort, 30*time.Second); err != nil {
		t.Fatalf("sshd did not come back after host key rotation: %v", err)
	}

	err = workspacessh.TestAuthDial(context.Background(),
		&recordingUI{}, uri, opts)
	require.Error(t, err)
	assert.True(t, errors.Is(err, workspacessh.ErrHostKeyMismatch),
		"after the server rotates its host key, a re-dial against "+
			"the same known_hosts entry must surface "+
			"ErrHostKeyMismatch — otherwise users would silently "+
			"connect to a server they no longer trust; got %v", err)
}
