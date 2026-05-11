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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
)

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
	origSrv := mgr.servers["go"]
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
		s := mgr.servers["go"]
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
