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

package idelsp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// TestEventTypeClose_evictsFilesCache locks in the fix for RUNE-195:
// Manager.files retained the full text of every URI ever opened in a
// language-supported buffer because EventTypeClose only emitted
// textDocument/didClose without touching m.files. The handler must
// drop the cached entry so long-running sessions do not accumulate
// one full file copy per URI.
func TestEventTypeClose_evictsFilesCache(t *testing.T) {
	t.Parallel()
	workspaceURI := makeURI(t, "file:///workspace")
	m := New(workspaceURI, nil, nil, nil, nil, nil,
		Config{NoInitializeServer: true})
	t.Cleanup(func() { _ = m.Close() })

	openURI := makeURI(t, "file:///workspace/open.go")
	m.mu.Lock()
	m.files[openURI.String()] = newFile(openURI, "package open\n", "go",
		serverKey{languageID: "go", rootURI: "file:///workspace"})
	m.mu.Unlock()

	require.Len(t, m.files, 1)

	err := m.handle(textapi.Event{
		Type: textapi.EventTypeClose,
		URI:  openURI,
	})
	require.NoError(t, err)

	m.mu.Lock()
	defer m.mu.Unlock()
	assert.Empty(t, m.files,
		"EventTypeClose must evict the cached file entry")
}

// TestManagerConcurrentStateAccess drives the file/server/pendingOpens
// accessors and Close concurrently to prove m.mu serialises every read
// and write of the manager's maps. With NoInitializeServer the open/close
// events stay in-process (no language server is spawned), so this is a
// pure -race regression guard for the locking around m.files,
// m.pendingOpens, and m.servers.
func TestManagerConcurrentStateAccess(t *testing.T) {
	t.Parallel()
	workspaceURI := makeURI(t, "file:///workspace")
	m := New(workspaceURI, nil, nil, nil, nil, nil,
		Config{NoInitializeServer: true})

	const workers = 8
	files := []workspaceapi.URI{
		makeURI(t, "file:///workspace/a.go"),
		makeURI(t, "file:///workspace/b.go"),
		makeURI(t, "file:///workspace/c.go"),
	}

	var wg sync.WaitGroup
	for i := range workers {
		uri := files[i%len(files)]
		wg.Go(func() {
			for range 50 {
				_ = m.handle(textapi.Event{Type: textapi.EventTypeOpen, URI: uri,
					Content: "package a\n"})
				m.mu.Lock()
				m.files[uri.String()] = newFile(uri, "package a\n", "go",
					serverKey{languageID: "go", rootURI: "file:///workspace"})
				m.mu.Unlock()
				_, _ = m.getFile(uri.String())
				_ = m.allServers()
				_ = m.handle(textapi.Event{Type: textapi.EventTypeClose, URI: uri})
			}
		})
	}

	wg.Go(func() {
		_ = m.Close()
	})

	wg.Wait()
}

// TestManagerMaxRetriesPropagatesFromConfig locks in the fix for Bug B
// in RUNE-132: Config.MaxRetries was read for default-filling but
// never assigned to Manager.maxRetries, so retry.LimitStrategy
// silently received zero and watchServer never restarted a crashed
// language server.
func TestManagerMaxRetriesPropagatesFromConfig(t *testing.T) {
	uri := makeURI(t, "file:///workspace")

	t.Run("explicit value is propagated", func(t *testing.T) {
		m := New(uri, nil, nil, nil, nil, nil, Config{MaxRetries: 7})
		t.Cleanup(func() { _ = m.Close() })
		assert.Equal(t, uint(7), m.maxRetries)
	})

	t.Run("zero falls back to default", func(t *testing.T) {
		m := New(uri, nil, nil, nil, nil, nil, Config{})
		t.Cleanup(func() { _ = m.Close() })
		assert.Equal(t, uint(3), m.maxRetries)
	})
}

// TestDidChangeWatchedFiles_invalidatesOnDelete locks in the fix for
// RUNE-AGENT-72: a workspace/didChangeWatchedFiles batch that contains
// a Deleted event must call Callback.InvalidateAllPending so subsequent
// WaitFileProcessed calls block until gopls re-publishes diagnostics
// for unrelated files in the same package. Non-deletion batches must
// not trigger the invalidation.
func TestDidChangeWatchedFiles_invalidatesOnDelete(t *testing.T) {
	t.Parallel()
	uri := makeURI(t, "file:///workspace")

	t.Run("deletion event triggers InvalidateAllPending", func(t *testing.T) {
		t.Parallel()
		callback := &testCallback{}
		m := New(uri, nil, nil, nil, nil, nil,
			Config{Callback: callback, NoInitializeServer: true})
		t.Cleanup(func() { _ = m.Close() })

		err := m.DidChangeWatchedFiles(context.Background(),
			semanticapi.DidChangeWatchedFilesParams{
				Changes: []semanticapi.FileEvent{
					{URI: "file:///workspace/a.go", Type: semanticapi.FileChangeTypeChanged},
					{URI: "file:///workspace/b.go", Type: semanticapi.FileChangeTypeDeleted},
				},
			})
		require.NoError(t, err)

		callback.mu.Lock()
		got := callback.invalidateAllPendingCount
		callback.mu.Unlock()
		assert.Equal(t, 1, got)
	})

	t.Run("rename-as-delete+create triggers InvalidateAllPending", func(t *testing.T) {
		t.Parallel()
		callback := &testCallback{}
		m := New(uri, nil, nil, nil, nil, nil,
			Config{Callback: callback, NoInitializeServer: true})
		t.Cleanup(func() { _ = m.Close() })

		err := m.DidChangeWatchedFiles(context.Background(),
			semanticapi.DidChangeWatchedFilesParams{
				Changes: []semanticapi.FileEvent{
					{URI: "file:///workspace/old.go", Type: semanticapi.FileChangeTypeDeleted},
					{URI: "file:///workspace/new.go", Type: semanticapi.FileChangeTypeCreated},
				},
			})
		require.NoError(t, err)

		callback.mu.Lock()
		got := callback.invalidateAllPendingCount
		callback.mu.Unlock()
		assert.Equal(t, 1, got)
	})

	t.Run("add+change only does not trigger InvalidateAllPending", func(t *testing.T) {
		t.Parallel()
		callback := &testCallback{}
		m := New(uri, nil, nil, nil, nil, nil,
			Config{Callback: callback, NoInitializeServer: true})
		t.Cleanup(func() { _ = m.Close() })

		err := m.DidChangeWatchedFiles(context.Background(),
			semanticapi.DidChangeWatchedFilesParams{
				Changes: []semanticapi.FileEvent{
					{URI: "file:///workspace/a.go", Type: semanticapi.FileChangeTypeCreated},
					{URI: "file:///workspace/b.go", Type: semanticapi.FileChangeTypeChanged},
				},
			})
		require.NoError(t, err)

		callback.mu.Lock()
		got := callback.invalidateAllPendingCount
		callback.mu.Unlock()
		assert.Equal(t, 0, got)
	})
}

// TestDidChangeWatchedFiles_marksOpenFilePending locks in the fix for
// RUNE-AGENT-72 (round 2): when an apply_patch-style Changed event
// arrives for a file that is also open in the editor, the manager
// must still mark the URI as pending so a subsequent
// WaitFileProcessed blocks until gopls re-publishes diagnostics.
// Previously fileDidChangeOOB short-circuited for open files,
// causing check_file_errors to return a stale pre-patch snapshot.
func TestDidChangeWatchedFiles_marksOpenFilePending(t *testing.T) {
	t.Parallel()
	uri := makeURI(t, "file:///workspace")
	callback := &testCallback{}
	m := New(uri, nil, nil, nil, nil, nil,
		Config{Callback: callback, NoInitializeServer: true})
	t.Cleanup(func() { _ = m.Close() })

	// Simulate the file being open in the editor.
	openURI := makeURI(t, "file:///workspace/open.go")
	m.mu.Lock()
	m.files[openURI.String()] = newFile(openURI, "package open\n", "go",
		serverKey{languageID: "go", rootURI: "file:///workspace"})
	m.mu.Unlock()

	// An apply_patch-style Changed event arrives for the open file.
	err := m.DidChangeWatchedFiles(context.Background(),
		semanticapi.DidChangeWatchedFilesParams{
			Changes: []semanticapi.FileEvent{
				{URI: openURI.String(), Type: semanticapi.FileChangeTypeChanged},
			},
		})
	require.NoError(t, err)

	// The callback must have received an out-of-band change marked
	// as open so a subsequent WaitFileProcessed blocks for a newer
	// version rather than releasing on a stale push.
	callback.mu.Lock()
	got := callback.fileDidChangeCalls
	callback.mu.Unlock()
	require.Len(t, got, 1)
	assert.Equal(t, openURI.String(), got[0].uri)
	assert.True(t, got[0].open)
	assert.True(t, got[0].oob)
}

// TestWatchServerRestartsOnConnLoss locks in the fix that watchServer
// listens on srv.conn.Done() in addition to the process watcher.
// Closing the IDE-side jsonrpc2 conn while the gopls process is still
// alive must trigger a SIGKILL of the orphan plus a fresh server.
func TestWatchServerRestartsOnConnLoss(t *testing.T) {
	t.Parallel()
	goplsBin := findGopls(t)
	tmpDir := setupTestWorkspace(t, "testdata")

	uri := makeURI(t, "file://"+tmpDir)
	scheme := newTestScheme()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var ready sync.Once
	var wg sync.WaitGroup
	callback := &testCallback{
		onProgress: readyOnProgress(&ready, &wg),
	}

	mgr := New(
		uri, scheme, scheme,
		&stubPkgManager{bin: goplsBin},
		nil, nil,
		Config{Callback: callback, MaxRetries: 3, NoInitializeServer: true},
	)
	t.Cleanup(func() { _ = mgr.Close() })

	params := autoInitParams(uri.String())
	initOpts, err := json.Marshal(map[string]any{
		"langID":  "go",
		"command": "gopls serve",
	})
	require.NoError(t, err)
	params.InitializeOptions = initOpts

	wg.Add(1)
	_, err = mgr.Initialize(ctx, params)
	require.NoError(t, err)
	wg.Wait()

	mgr.mu.Lock()
	origSrv := mgr.servers[serverKey{languageID: "go", rootURI: uri.String()}].(*langServer)
	mgr.mu.Unlock()
	require.NotNil(t, origSrv)
	origPid := origSrv.pid

	// Tear down the jsonrpc2 connection without touching the
	// child process. Closing the IDE-side stdin/stdout breaks
	// the readIncoming goroutine, which closes the conn's done
	// channel.
	require.NoError(t, origSrv.stdin.Close())
	require.NoError(t, origSrv.stdout.Close())

	require.Eventually(t, func() bool {
		select {
		case <-origSrv.conn.Done():
			return true
		default:
			return false
		}
	}, 5*time.Second, 50*time.Millisecond,
		"original conn never reported Done")

	// The manager must now kill the orphan process and start a new server.
	var newSrv *langServer
	require.Eventually(t, func() bool {
		mgr.mu.Lock()
		s, _ := mgr.servers[serverKey{languageID: "go", rootURI: uri.String()}].(*langServer)
		mgr.mu.Unlock()
		if s != nil && s != origSrv {
			newSrv = s
			return true
		}
		return false
	}, 15*time.Second, 200*time.Millisecond,
		"server did not restart after conn loss")

	assert.NotEqual(t, origPid, newSrv.pid,
		"new server must have a different pid")
	assert.Equal(t, origSrv.params, newSrv.params,
		"InitializeParams must be preserved across conn-loss restart")

	// Sanity check the new server actually responds.
	pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
	defer pingCancel()
	var raw semanticapi.DocumentSymbolResult
	_ = newSrv.call(pingCtx, "workspace/symbol",
		semanticapi.WorkspaceSymbolParams{Query: ""}, &raw)
}

// TestManagerCloseTerminatesGopls locks in the RUNE-180 contract that
// Manager.Close propagates cancellation down to spawned language-server
// processes (m.ctx → langServer.ctx → exec.CommandContext-bound child).
func TestManagerCloseTerminatesGopls(t *testing.T) {
	t.Parallel()
	goplsBin := findGopls(t)
	tmpDir := setupTestWorkspace(t, "testdata")

	uri := makeURI(t, "file://"+tmpDir)
	scheme := newTestScheme()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var ready sync.Once
	var wg sync.WaitGroup
	callback := &testCallback{
		onProgress: readyOnProgress(&ready, &wg),
	}

	mgr := New(
		uri, scheme, scheme,
		&stubPkgManager{bin: goplsBin},
		nil, nil,
		Config{Callback: callback, MaxRetries: 3, NoInitializeServer: true},
	)

	params := autoInitParams(uri.String())
	initOpts, err := json.Marshal(map[string]any{
		"langID":  "go",
		"command": "gopls serve",
	})
	require.NoError(t, err)
	params.InitializeOptions = initOpts

	wg.Add(1)
	_, err = mgr.Initialize(ctx, params)
	require.NoError(t, err)
	wg.Wait()

	mgr.mu.Lock()
	srv := mgr.servers[serverKey{languageID: "go", rootURI: uri.String()}].(*langServer)
	mgr.mu.Unlock()
	require.NotNil(t, srv)
	require.NotNil(t, srv.watcher,
		"langServer must have a process watcher so close can be observed")

	require.NoError(t, mgr.Close())

	require.Error(t, mgr.ctx.Err(),
		"Manager.Close must cancel m.ctx so handleEvs / watchServer exit")

	select {
	case <-srv.watcher:
	case <-time.After(10 * time.Second):
		t.Fatal("gopls child did not exit within 10s of Manager.Close: " +
			"manager teardown is not propagating cancellation to the child")
	}
}

// newTransientTestManager builds a Manager backed by the local
// filesystem with a single fake python server rooted at rootPath.
func newTransientTestManager(
	t *testing.T, rootPath string,
) (*Manager, *fakeChild) {
	t.Helper()
	rootURI := "file://" + rootPath
	uri := makeURI(t, rootURI)
	m := New(uri, newTestScheme(), nil, nil, nil, nil,
		Config{NoInitializeServer: true})
	t.Cleanup(func() { _ = m.Close() })

	srv := &fakeChild{childName: "python"}
	m.mu.Lock()
	m.servers[serverKey{languageID: "python", rootURI: rootURI}] = srv
	m.mu.Unlock()
	return m, srv
}

// TestTransientOpenForUnopenedFile asserts that a position request for
// a file not tracked in m.files transiently opens it (didOpen) before
// the call and closes it (didClose) after, without caching it.
func TestTransientOpenForUnopenedFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "mod.py")
	require.NoError(t, os.WriteFile(filePath, []byte("x = 1"), 0o644))
	fileURI := "file://" + filePath

	m, srv := newTransientTestManager(t, tmpDir)

	_, err := m.Definition(context.Background(), semanticapi.DefinitionParams{
		TextDocument: semanticapi.TextDocumentIdentifier{URI: fileURI},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{
		"notify:textDocument/didOpen",
		"call:textDocument/definition",
		"notify:textDocument/didClose",
	}, srv.eventLog(), "didOpen must precede the call and didClose must follow")

	opens := srv.didOpens()
	require.Len(t, opens, 1)
	assert.Equal(t, fileURI, opens[0].TextDocument.URI)
	assert.Equal(t, "python", opens[0].TextDocument.LanguageID)
	assert.Equal(t, int32(firstFileVersion), opens[0].TextDocument.Version)
	assert.Equal(t, "x = 1\n", opens[0].TextDocument.Text)

	m.mu.Lock()
	_, cached := m.files[fileURI]
	m.mu.Unlock()
	assert.False(t, cached, "transient open must not cache the file in m.files")
}

// TestNoTransientOpenForOpenFile asserts a file already open in the
// editor (present in m.files) is queried directly, with no extra
// didOpen/didClose.
func TestNoTransientOpenForOpenFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "mod.py")
	require.NoError(t, os.WriteFile(filePath, []byte("x = 1"), 0o644))
	fileURI := "file://" + filePath

	m, srv := newTransientTestManager(t, tmpDir)
	key := serverKey{languageID: "python", rootURI: "file://" + tmpDir}
	uri := makeURI(t, fileURI)
	m.mu.Lock()
	m.files[fileURI] = newFile(uri, "x = 1\n", "python", key)
	m.mu.Unlock()

	_, err := m.Definition(context.Background(), semanticapi.DefinitionParams{
		TextDocument: semanticapi.TextDocumentIdentifier{URI: fileURI},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{"call:textDocument/definition"}, srv.eventLog(),
		"an already-open file must not trigger didOpen/didClose")
}

// TestTransientOpenReadErrorSurfaces asserts a missing file surfaces
// the read error without sending any notification or call.
func TestTransientOpenReadErrorSurfaces(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	fileURI := "file://" + filepath.Join(tmpDir, "missing.py")

	m, srv := newTransientTestManager(t, tmpDir)

	_, err := m.Definition(context.Background(), semanticapi.DefinitionParams{
		TextDocument: semanticapi.TextDocumentIdentifier{URI: fileURI},
	})
	require.Error(t, err)
	assert.Empty(t, srv.eventLog(),
		"a read error must fail before any didOpen or call")
}

// TestTransientOpenNoServerPreservesErrNoServer asserts routing errors
// are returned unchanged when no server owns the file's language.
func TestTransientOpenNoServerPreservesErrNoServer(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	goURI := "file://" + filepath.Join(tmpDir, "main.go")

	m, _ := newTransientTestManager(t, tmpDir)

	_, err := m.Definition(context.Background(), semanticapi.DefinitionParams{
		TextDocument: semanticapi.TextDocumentIdentifier{URI: goURI},
	})
	require.ErrorIs(t, err, ErrNoServer)
}

// TestTransientOpenWrapsDirectCallMethods asserts methods that call the
// server directly (Hover) also transiently open the file.
func TestTransientOpenWrapsDirectCallMethods(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "mod.py")
	require.NoError(t, os.WriteFile(filePath, []byte("x = 1"), 0o644))
	fileURI := "file://" + filePath

	m, srv := newTransientTestManager(t, tmpDir)

	_, err := m.Hover(context.Background(), semanticapi.HoverParams{
		TextDocument: semanticapi.TextDocumentIdentifier{URI: fileURI},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{
		"notify:textDocument/didOpen",
		"call:textDocument/hover",
		"notify:textDocument/didClose",
	}, srv.eventLog())
}

// TestTransientOpenWrapsDocumentSymbol asserts DocumentSymbol, a
// document-scoped request, transiently opens a file that is not already
// open so servers like ty (which reject requests on unopened documents)
// can answer it.
func TestTransientOpenWrapsDocumentSymbol(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "mod.py")
	require.NoError(t, os.WriteFile(filePath, []byte("x = 1"), 0o644))
	fileURI := "file://" + filePath

	m, srv := newTransientTestManager(t, tmpDir)

	_, err := m.DocumentSymbol(context.Background(), semanticapi.DocumentSymbolParams{
		TextDocument: semanticapi.TextDocumentIdentifier{URI: fileURI},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{
		"notify:textDocument/didOpen",
		"call:textDocument/documentSymbol",
		"notify:textDocument/didClose",
	}, srv.eventLog())

	m.mu.Lock()
	_, cached := m.files[fileURI]
	m.mu.Unlock()
	assert.False(t, cached, "transient open must not cache the file in m.files")
}

// TestDiagnosticSettleTimeoutDoesNotFail asserts that a tracked file for
// which the server never pushes publishDiagnostics does not make
// Diagnostic fail with the caller's deadline. WaitFileProcessed would
// otherwise block until the caller's context expires; the settle wait
// must instead fall through to the pull request. This reproduces the
// check_file_errors DeadlineExceeded seen with ty for unopened files.
func TestDiagnosticSettleTimeoutDoesNotFail(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "mod.py")
	require.NoError(t, os.WriteFile(filePath, []byte("x = 1"), 0o644))
	fileURI := "file://" + filePath
	rootURI := "file://" + tmpDir

	cb := NewCallbackHandler(nil, nil, nil, nil, nil, rootURI,
		CallbackHandlerConfig{ScheduleNextTick: func(func()) bool { return true }})
	uri := makeURI(t, rootURI)
	m := New(uri, newTestScheme(), nil, nil, nil, nil,
		Config{NoInitializeServer: true, Callback: cb})
	t.Cleanup(func() { _ = m.Close() })
	srv := &fakeChild{childName: "python"}
	m.mu.Lock()
	m.servers[serverKey{languageID: "python", rootURI: rootURI}] = srv
	m.mu.Unlock()

	// The file must be open in the editor for the settle wait to run at
	// all; an unopened file skips it (see
	// TestDiagnosticUnopenedFileSkipsSettleWait).
	fileURIParsed := makeURI(t, fileURI)
	m.mu.Lock()
	m.files[fileURI] = newFile(fileURIParsed, "x = 1\n", "python",
		serverKey{languageID: "python", rootURI: rootURI})
	m.mu.Unlock()

	// Track a pending version the server will never acknowledge with a
	// publishDiagnostics push, so WaitFileProcessed would block.
	cb.FileDidChange(fileURI, 1, true, false)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := time.Now()
	_, err := m.Diagnostic(ctx, semanticapi.DocumentDiagnosticParams{
		TextDocument: semanticapi.TextDocumentIdentifier{URI: fileURI},
	})
	require.NoError(t, err)
	assert.Less(t, time.Since(start), 10*time.Second,
		"Diagnostic must not block on the full caller deadline waiting "+
			"for a publishDiagnostics that never arrives")
	assert.Contains(t, srv.eventLog(), "call:textDocument/diagnostic",
		"the pull request must still be issued after the settle wait")
}

// TestDiagnosticUnopenedFileSkipsSettleWait asserts that a file which
// is not open in the editor does not pay the settle-wait timeout: the
// pull runs immediately against a transient didOpen carrying fresh disk
// content. Without this, ty (which never pushes publishDiagnostics for
// untracked files) would make check_file_errors after apply_patch stall
// for the whole diagnosticSettleTimeout on every edit.
func TestDiagnosticUnopenedFileSkipsSettleWait(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "mod.py")
	require.NoError(t, os.WriteFile(filePath, []byte("x = 1"), 0o644))
	fileURI := "file://" + filePath
	rootURI := "file://" + tmpDir

	cb := NewCallbackHandler(nil, nil, nil, nil, nil, rootURI,
		CallbackHandlerConfig{ScheduleNextTick: func(func()) bool { return true }})
	uri := makeURI(t, rootURI)
	m := New(uri, newTestScheme(), nil, nil, nil, nil,
		Config{NoInitializeServer: true, Callback: cb})
	t.Cleanup(func() { _ = m.Close() })
	srv := &fakeChild{childName: "python"}
	m.mu.Lock()
	m.servers[serverKey{languageID: "python", rootURI: rootURI}] = srv
	m.mu.Unlock()

	// Mark a pending version the server will never acknowledge. Because
	// the file is not open, the settle wait must be skipped entirely
	// rather than waiting out diagnosticSettleTimeout.
	cb.FileDidChange(fileURI, 1, true, false)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := time.Now()
	_, err := m.Diagnostic(ctx, semanticapi.DocumentDiagnosticParams{
		TextDocument: semanticapi.TextDocumentIdentifier{URI: fileURI},
	})
	require.NoError(t, err)
	assert.Less(t, time.Since(start), diagnosticSettleTimeout,
		"Diagnostic for an unopened file must not pay the settle wait")
	// Transient open/close must bracket the pull, and the pull must run.
	assert.Equal(t, []string{
		"notify:textDocument/didOpen",
		"call:textDocument/diagnostic",
		"notify:textDocument/didClose",
	}, srv.eventLog())
}

// TestDiagnosticPropagatesCancellation asserts that a genuinely
// cancelled caller context aborts Diagnostic rather than proceeding to
// the pull request.
func TestDiagnosticPropagatesCancellation(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "mod.py")
	require.NoError(t, os.WriteFile(filePath, []byte("x = 1"), 0o644))
	fileURI := "file://" + filePath
	rootURI := "file://" + tmpDir

	cb := NewCallbackHandler(nil, nil, nil, nil, nil, rootURI,
		CallbackHandlerConfig{ScheduleNextTick: func(func()) bool { return true }})
	uri := makeURI(t, rootURI)
	m := New(uri, newTestScheme(), nil, nil, nil, nil,
		Config{NoInitializeServer: true, Callback: cb})
	t.Cleanup(func() { _ = m.Close() })
	srv := &fakeChild{childName: "python"}
	m.mu.Lock()
	m.servers[serverKey{languageID: "python", rootURI: rootURI}] = srv
	m.mu.Unlock()

	cb.FileDidChange(fileURI, 1, true, false)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := m.Diagnostic(ctx, semanticapi.DocumentDiagnosticParams{
		TextDocument: semanticapi.TextDocumentIdentifier{URI: fileURI},
	})
	require.ErrorIs(t, err, context.Canceled)
}
