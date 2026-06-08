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

package exoeditor

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/workspace"
)

// countingNotifications records how many warn-level notifications the
// watcher emitted so tests can assert the missing-file case does not
// surface a "watch ... no such file" error to the user.
type countingNotifications struct {
	mu   sync.Mutex
	warn int
}

func (n *countingNotifications) Notify(
	level browserapi.NotificationLevel, _ string, _ ...any,
) (string, error) {
	if level == browserapi.LevelWarn {
		n.mu.Lock()
		n.warn++
		n.mu.Unlock()
	}
	return "", nil
}

func (n *countingNotifications) NotifyOnce(
	browserapi.NotificationLevel, string, ...any,
) (string, error) {
	return "", nil
}

func (n *countingNotifications) UpdateNotificationProgress(
	string, string, int64, int64,
) error {
	return nil
}

func (n *countingNotifications) warnCount() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.warn
}

// syncReloader records reload URIs under a mutex; the watcher goroutine
// and the test goroutine both touch it.
type syncReloader struct {
	mu    sync.Mutex
	calls []workspaceapi.URI
}

func (r *syncReloader) Reload(uri workspaceapi.URI) error {
	r.mu.Lock()
	r.calls = append(r.calls, uri)
	r.mu.Unlock()
	return nil
}

func (r *syncReloader) reloadCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func newFileSchemeWorkspace(t *testing.T, dir string) (workspace.Workspace, schemeapi.Scheme) {
	t.Helper()
	wsURI, err := workspaceapi.ParseURI("file://" + dir)
	require.NoError(t, err)
	scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), wsURI)
	require.NoError(t, err)
	t.Cleanup(func() { _ = scheme.Close() })
	ws := workspace.NewSchemeWorkspace(wsURI, scheme, func(fn func()) bool { fn(); return true })
	return ws, scheme
}

// TestStartWatcherFallsBackToParentDirForMissingFile reproduces the
// bug where opening a brand-new file under the external editor failed
// with "watch <path>: notify: lstat <path>: no such file or
// directory" because the notify backend lstats the watched path at
// registration time. The watcher must instead watch the parent
// directory, stay active, suppress the warning, and still fire a
// reload once the file is created.
func TestStartWatcherFallsBackToParentDirForMissingFile(t *testing.T) {
	dir := t.TempDir()
	ws, _ := newFileSchemeWorkspace(t, dir)

	fpath := filepath.Join(dir, "new.txt")
	fileURI, err := workspaceapi.ParseURI("file://" + fpath)
	require.NoError(t, err)

	notes := &countingNotifications{}
	rel := &syncReloader{}
	h := &editorHandler{
		resource:         fileURI,
		cwd:              ws,
		notifications:    notes,
		scheduleNextTick: func(fn func()) bool { fn(); return true },
		reloader:         rel,
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h.startWatcher(ctx)

	require.True(t, h.watchActive,
		"watcher must stay active for a not-yet-created file")
	require.Equal(t, 0, notes.warnCount(),
		"opening a missing file must not surface a watch warning")

	require.NoError(t, os.WriteFile(fpath, []byte("hello\n"), 0o644))

	require.Eventually(t, func() bool {
		return rel.reloadCount() > 0
	}, 5*time.Second, 10*time.Millisecond,
		"creating the watched file must trigger a reload")

	rel.mu.Lock()
	first := rel.calls[0]
	rel.mu.Unlock()
	assert.Equal(t, fileURI, first,
		"reload must target the resource URI, not the symlink-resolved event path")
}

// TestStartWatcherDirModeIgnoresSiblingFiles confirms the parent-dir
// fallback filters events down to the resource: writes to sibling
// files in the same directory must not trigger a reload of the open
// file.
func TestStartWatcherDirModeIgnoresSiblingFiles(t *testing.T) {
	dir := t.TempDir()
	ws, _ := newFileSchemeWorkspace(t, dir)

	fpath := filepath.Join(dir, "target.txt")
	fileURI, err := workspaceapi.ParseURI("file://" + fpath)
	require.NoError(t, err)

	rel := &syncReloader{}
	h := &editorHandler{
		resource:         fileURI,
		cwd:              ws,
		notifications:    &countingNotifications{},
		scheduleNextTick: func(fn func()) bool { fn(); return true },
		reloader:         rel,
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h.startWatcher(ctx)
	require.True(t, h.watchActive)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.txt"),
		[]byte("noise\n"), 0o644))

	// Give the watcher a chance to (incorrectly) react before
	// asserting it stayed quiet.
	assert.Never(t, func() bool {
		return rel.reloadCount() > 0
	}, 500*time.Millisecond, 25*time.Millisecond,
		"writes to sibling files must not reload the open file")
}

// TestStartWatcherDirectModeReloadsResourceURI covers the existing-file
// path: the notify backend reports the symlink-resolved path
// (/private/tmp/... for /tmp/...), so reloading the raw event URI would
// miss the tab keyed by the original resource URI and surface
// "cannot reload this content". The reload must target the resource.
func TestStartWatcherDirectModeReloadsResourceURI(t *testing.T) {
	dir := t.TempDir()
	ws, _ := newFileSchemeWorkspace(t, dir)

	fpath := filepath.Join(dir, "exists.txt")
	require.NoError(t, os.WriteFile(fpath, []byte("initial\n"), 0o644))
	fileURI, err := workspaceapi.ParseURI("file://" + fpath)
	require.NoError(t, err)

	rel := &syncReloader{}
	h := &editorHandler{
		resource:         fileURI,
		cwd:              ws,
		notifications:    &countingNotifications{},
		scheduleNextTick: func(fn func()) bool { fn(); return true },
		reloader:         rel,
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h.startWatcher(ctx)
	require.True(t, h.watchActive)

	require.NoError(t, os.WriteFile(fpath, []byte("updated\n"), 0o644))

	require.Eventually(t, func() bool {
		return rel.reloadCount() > 0
	}, 5*time.Second, 10*time.Millisecond,
		"writing the watched file must trigger a reload")

	rel.mu.Lock()
	first := rel.calls[0]
	rel.mu.Unlock()
	assert.Equal(t, fileURI, first,
		"reload must target the resource URI, not the symlink-resolved event path")
}
