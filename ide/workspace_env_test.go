// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.

package ide

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

func mustFileURI(t *testing.T, path string) workspaceapi.URI {
	t.Helper()
	u, err := workspaceapi.ParseURI("file://" + path)
	require.NoError(t, err)
	return u
}

func TestWorkspaceBasename(t *testing.T) {
	cases := []struct {
		uri  string
		want string
	}{
		{"file:///Users/alice/projects/blue", "blue"},
		{"file:///Users/alice/work/blue/", "blue"},
		{"file:///tmp/has space/foo bar", "foo_bar"},
		{"file:///tmp/weird:name", "weird_name"},
		// Non-file schemes resolve too: commands dispatched through
		// an alias run via the workspace's executor in its native
		// filesystem, so the URI path is a meaningful identifier.
		{"ssh://user@host/srv/blue", "blue"},
		{"memory:///tmp/playground/blue", "blue"},
	}
	for _, tc := range cases {
		uri, err := workspaceapi.ParseURI(tc.uri)
		require.NoError(t, err, "uri %q", tc.uri)
		got := workspaceBasename(uri)
		assert.Equal(t, tc.want, got, "uri %q", tc.uri)
	}
}

// TestWorkspaceEnvVarsAreStableAndPerPath asserts that two workspaces
// sharing a basename but mounted at different absolute paths derive
// different $WORKSPACE_HASH values, so derived directories produced
// from `$WORKSPACE-$WORKSPACE_HASH` don't collide on disk.
func TestWorkspaceEnvVarsAreStableAndPerPath(t *testing.T) {
	a := mustFileURI(t, "/Users/alice/projects/blue")
	b := mustFileURI(t, "/Users/alice/work/blue")

	baseA := workspaceBasename(a)
	baseB := workspaceBasename(b)
	assert.Equal(t, "blue", baseA)
	assert.Equal(t, baseA, baseB)

	h1 := workspaceHash(a)
	h2 := workspaceHash(b)
	assert.NotEqual(t, h1, h2,
		"workspaces with same basename but different paths must "+
			"have distinct hashes so derived paths do not collide")
	re := regexp.MustCompile(`^[0-9a-f]{4}$`)
	assert.Regexp(t, re, h1)
	assert.Regexp(t, re, h2)

	// determinism
	h1b := workspaceHash(a)
	assert.Equal(t, h1, h1b)

	// trailing slash and `.` segments collapse via filepath.Clean
	h1c := workspaceHash(mustFileURI(t,
		"/Users/alice/projects/./blue/"))
	assert.Equal(t, h1, h1c)

	// Same path served by different schemes/hosts must hash
	// differently — the hash key includes scheme and host.
	sshA, err := workspaceapi.ParseURI(
		"ssh://alice@host-a/srv/blue")
	require.NoError(t, err)
	sshB, err := workspaceapi.ParseURI(
		"ssh://alice@host-b/srv/blue")
	require.NoError(t, err)
	assert.NotEqual(t, workspaceHash(sshA), workspaceHash(sshB))
}

// TestWorkspaceManagerHandlerEnvSource asserts the handler exposes
// all four workspace-scoped variables for every scheme. Commands run
// in the workspace's executor, so the variables must resolve whether
// the workspace is local, remote, or in-memory.
func TestWorkspaceManagerHandlerEnvSource(t *testing.T) {
	t.Run("memory test workspace exposes every workspace var", func(t *testing.T) {
		// The IDE test harness installs workspaces with the
		// memory:// scheme. Every $WORKSPACE_* variable resolves
		// because commands dispatched through these aliases would
		// run inside the workspace's own executor.
		dir := t.TempDir()
		m := newTestWorkspaceManagerHandlerWithDir(t,
			defaultConfigWithWrap(false), dir, nopShutdownShaderConfig())
		t.Cleanup(func() { _ = m.Close() })
		m.drainPendingWorkspaces()

		base, ok := m.envSource("WORKSPACE")
		assert.True(t, ok)
		assert.NotEmpty(t, base)
		hash, ok := m.envSource("WORKSPACE_HASH")
		assert.True(t, ok)
		assert.Regexp(t, `^[0-9a-f]{4}$`, hash)
		uriStr, ok := m.envSource("WORKSPACE_URI")
		assert.True(t, ok)
		assert.NotEmpty(t, uriStr)
		assert.Contains(t, uriStr, dir)
		path, ok := m.envSource("WORKSPACE_PATH")
		assert.True(t, ok)
		assert.Equal(t, dir, path)
	})

	t.Run("unknown names return ok=false", func(t *testing.T) {
		dir := t.TempDir()
		m := newTestWorkspaceManagerHandlerWithDir(t,
			defaultConfigWithWrap(false), dir, nopShutdownShaderConfig())
		t.Cleanup(func() { _ = m.Close() })
		_, ok := m.envSource("FILE")
		assert.False(t, ok, "envSource is workspace-scoped only")
		_, ok = m.envSource("")
		assert.False(t, ok)
	})
}
