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

package workspacetest

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/ide"
)

// TestIntegrationIDEWorkspaceReloadOverSSH reproduces the user-visible
// bug where `:workspacereload` against an SSH-backed workspace leaves
// every subsequent scheme operation (open file, open pty, stat, ...)
// returning "context canceled".
//
// Original root cause: commandReloadWorkspace used to close the
// workspaceHandler via a "keep scheme" path that called
// hm.cancelCtx() — the pending workspace's cancel func. That cancel
// propagated through newRemoteScheme's WithCancel into the
// remoteScheme.ctx that drives every gRPC call. addWorkspace then
// ran again, but workspace.Manager.AddWorkspace returned the *same*
// managerWorkspace from its cache, so the freshly-installed
// workspaceHandler wrapped the *original* remoteScheme — now with
// a permanently-canceled context.
//
// Post RUNE-180 fix: commandReloadWorkspace now routes through
// closeWorkspace, which calls closeAndRemove. The scheme is closed
// and dropped from workspace.Manager's cache, so addWorkspace builds
// a fresh remoteScheme on reopen and this test exercises the
// close+open cycle end-to-end.
//
// The test boots an IDE pointed at ssh://test@<container>/tmp/<dir>,
// waits for the cwd workspace install, then issues
// `:workspacereload<enter>` through the handler and asserts that
// post-reload Open on the same workspace still works.
func TestIntegrationIDEWorkspaceReloadOverSSH(t *testing.T) {
	SkipIfNoDocker(t)
	EnsureImage(t)

	c := StartContainer(t, SSHDScenario{
		PublicKeyFile:     "/id_ed25519.pub",
		InstallRuneBinary: true,
	})
	keyPath := PrivateKeyPath(t, "id_ed25519")

	// Stage a workspace dir + two files on the remote: one we open
	// before the reload, one we open after. The IDE's bootstrap
	// expects the workspace path to already exist.
	const remoteDir = "/tmp/rune_reload_test"
	workspaceURI, err := workspaceapi.ParseURI(fmt.Sprintf(
		"ssh://test@%s%s", c.HostPort, remoteDir))
	require.NoError(t, err)
	stageRemoteWorkspace(t, c.HostPort, keyPath, workspaceURI,
		"before.txt", "after.txt")

	// Local IDE config: modal mode + ":" as the command key so we
	// can drive `:workspacereload<enter>` through the handler.
	// workspace.ssh wires the stdlib remote against the container.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "rune.yaml")
	cfgYAML := fmt.Sprintf(`editor:
  mode: modal
command:
  key: ":"
workspace:
  ssh:
    private_keys: ["%s"]
    timeout: "20s"
    insecure: true
`, keyPath)
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgYAML), 0o644))

	var mu sync.Mutex
	i, err := ide.New(workspaceURI.String(), cfgPath, dir,
		ide.WithLocker(&mu),
		ide.WithScheduleNextTick(hostScheduleNextTickIDE(&mu)),
		ide.WithPublishEvent(func(term.Event) bool { return true }),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = i.Close() })

	rootHandler := i.Ready()
	// Phase B does the docker round trip + ssh dial in a goroutine
	// — wait for it to land before driving any remote operation.
	i.WaitWorkspaces()

	beforeURI, err := workspaceapi.ParseURI(fmt.Sprintf(
		"ssh://test@%s%s/before.txt", c.HostPort, remoteDir))
	require.NoError(t, err)
	afterURI, err := workspaceapi.ParseURI(fmt.Sprintf(
		"ssh://test@%s%s/after.txt", c.HostPort, remoteDir))
	require.NoError(t, err)

	// Sanity check: pre-reload the scheme is healthy. If this
	// fails the test environment itself is broken (image, dial,
	// rune binary install) — not the reload bug.
	pollOpenURI(t, &mu, i, beforeURI,
		"baseline open before reload should succeed")

	// Drive `:workspacereload<enter>` through the handler exactly
	// like a user would.
	feedKeys(t, &mu, rootHandler, ":workspacereload<enter>")
	i.WaitWorkspaces()

	// Post-reload Open must succeed. With the bug in place it
	// surfaces "context canceled" because the cached
	// schemeWorkspace still wraps the original remoteScheme whose
	// ctx was canceled by the close-half of reload.
	mu.Lock()
	openErr := i.Open(afterURI)
	mu.Unlock()
	require.NoError(t, openErr,
		"opening a remote file on the post-reload scheme must "+
			"not return context.Canceled (regression: "+
			":workspacereload over ssh)")
}

// hostScheduleNextTickIDE mirrors hostScheduleNextTick from the IDE
// e2e tests: dispatch UserFuncs on a fresh goroutine while holding
// the same locker the host event loop would. Required so the IDE's
// async install (Phase C) lands under the lock.
func hostScheduleNextTickIDE(mu sync.Locker) func(func()) bool {
	return func(fn func()) bool {
		go func() {
			mu.Lock()
			defer mu.Unlock()
			fn()
		}()
		return true
	}
}

// stageRemoteWorkspace mkdir -p's dirURI on the remote and creates
// each named file (empty). It uses a short-lived ssh scheme of its
// own — the same code path the IDE will hit when it boots — so the
// staging itself doubles as a smoke test that the container is
// reachable before we even invoke the IDE.
func stageRemoteWorkspace(
	t *testing.T, hostPort, keyPath string,
	dirURI workspaceapi.URI, files ...string,
) {
	t.Helper()
	cfg := config.MapConfig(map[string]any{
		"private_keys": []any{keyPath},
		"timeout":      "20s",
		"insecure":     true,
	})
	scheme := newSchemeIntegration(t, hostPort, cfg)
	require.NoError(t, scheme.MkdirAll(dirURI.Path(), 0o755))
	for _, name := range files {
		f, err := scheme.Create(filepath.Join(dirURI.Path(), name))
		require.NoError(t, err, "stage remote file %s", name)
		require.NoError(t, f.Close())
	}
}

// pollOpenURI polls i.Open until the remote scheme is connected
// enough to satisfy the editFileURI flow. The remote scheme dials
// asynchronously after newScheme returns, so the first attempt may
// race against the dial even after WaitWorkspaces returns.
func pollOpenURI(
	t *testing.T, mu *sync.Mutex, i *ide.IDE,
	uri workspaceapi.URI, msg string,
) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		mu.Lock()
		err := i.Open(uri)
		mu.Unlock()
		if err == nil {
			return
		}
		lastErr = err
		time.Sleep(200 * time.Millisecond)
	}
	require.NoError(t, lastErr, msg)
}

// feedKeys parses sequence (handlertest.SequenceTestCase format) and
// dispatches each key as a term.EventKey through h. The mu lock
// matches what the production event loop (gui / tui) holds around
// each Handle call, so handlers see the same visibility guarantees
// in test as they do at runtime.
func feedKeys(t *testing.T, mu *sync.Mutex, h tui.Handler, sequence string) {
	t.Helper()
	keys, err := term.ParseKeys(sequence)
	require.NoError(t, err, "parse %q", sequence)
	for _, key := range keys {
		ev := term.Event{
			Type: term.EventKey,
			Ch:   key.Ch,
			Mod:  key.Mod,
			Key:  key.Key,
		}
		switch {
		case ev.Ch != 0:
			ev.Raw = []byte(string(ev.Ch))
		case ev.Key == term.KeySpace:
			ev.Raw = []byte(" ")
		case ev.Key == term.KeyEnter:
			ev.Raw = []byte{0x0d, 0x0a}
		}
		mu.Lock()
		_, handled := h.Handle(ev)
		mu.Unlock()
		require.True(t, handled, "%s in %q",
			key.String(), sequence)
	}
}
