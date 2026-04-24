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


package idedebug

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-dap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/workspace/processctx"
)

func TestCloseConnLeavesAliveTrue(t *testing.T) {
	t.Parallel()

	c1, c2 := net.Pipe()
	defer c2.Close() //nolint:errcheck

	srv := &debugServer{
		conn:    c1,
		alive:   true,
		pending: make(map[int]chan dap.Message),
	}

	srv.closeConn()

	assert.False(t, srv.alive,
		"closeConn should set alive = false to prevent writes to a closed connection")
}

func TestWatchServerNotRetainsDeadServer(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mgr := &Manager{
		cfg: Config{
			MaxRetries:        1,
			InitializeTimeout: 500 * time.Millisecond,
			CloseTimeout:      time.Second,
		},
		servers:  make(map[string]*debugServer),
		starting: make(map[string]chan struct{}),
		eventSub: nopEventSubscriber{},
		executor: failingExecutor{},
		ctx:      ctx,
		cancel:   cancel,
		log:      slog.Default(),
	}

	watchCh := make(chan error, 1)
	c1, c2 := net.Pipe()
	defer c2.Close() //nolint:errcheck

	srvCtx, srvCancel := context.WithCancel(ctx)
	defer srvCancel()

	srv := &debugServer{
		ctx:     srvCtx,
		cancel:  srvCancel,
		conn:    c1,
		alive:   true,
		watcher: watchCh,
		cfg:     debugConfig{id: "test-adapter"},
		pending: make(map[int]chan dap.Message),
		log:     slog.With("test", true),
	}

	mgr.mu.Lock()
	mgr.servers["test-adapter"] = srv
	mgr.mu.Unlock()

	// Simulate a server crash.
	watchCh <- errors.New("process crashed")

	done := make(chan struct{})
	go func() {
		mgr.watchServer(
			&debugConfig{id: "test-adapter"},
			srv,
		)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("watchServer did not complete")
	}

	// Bug: watchServer logs the failure but doesn't remove
	// the dead server from m.servers. getOrCreateServer sees
	// the entry and returns it without any health check.
	mgr.mu.Lock()
	_, exists := mgr.servers["test-adapter"]
	mgr.mu.Unlock()

	assert.False(t, exists,
		"dead server should be removed from m.servers after failed retries")
}

func TestDebugServerStartCarriesProcessContext(t *testing.T) {
	startErr := errors.New("start failed")
	exec := &recordingDebugExecutor{err: startErr}
	srv := newDebugServer(
		context.Background(),
		debugConfig{id: "go", command: "dlv", args: []string{"dap", "--listen", "{addr}"}},
		"dlv",
		exec,
		"file:///workspace",
		nopEventSubscriber{},
	)

	ctx := processctx.ContextWithExtensionID(context.Background(), "go")
	err := srv.start(ctx)
	require.ErrorIs(t, err, startErr)

	extensionID, ok := processctx.ExtensionIDFromContext(exec.ctx)
	require.True(t, ok)
	assert.Equal(t, "go", extensionID)
	assert.Equal(t, "dlv", exec.cmd.Path)
	assert.Len(t, exec.cmd.Args, 3)
	assert.Equal(t, "dap", exec.cmd.Args[0])
	assert.Equal(t, "--listen", exec.cmd.Args[1])
	assert.NotEqual(t, "{addr}", exec.cmd.Args[2])
}

// failingExecutor always fails to start commands.
// Used to force retry failure in watchServer tests.
type failingExecutor struct{}

var _ schemeapi.Executor = failingExecutor{}

func (failingExecutor) StartCommand(
	context.Context, workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	return 0, errors.New("executor: forced failure")
}

func (failingExecutor) Signal(
	workspaceapi.Pid, syscall.Signal,
) error {
	return errors.New("not implemented")
}

func (failingExecutor) Close() error {
	return nil
}

type recordingDebugExecutor struct {
	ctx context.Context
	cmd workspaceapi.Cmd
	err error
}

func (e *recordingDebugExecutor) StartCommand(
	ctx context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	e.ctx = ctx
	e.cmd = cmd
	return 0, e.err
}

func (e *recordingDebugExecutor) Signal(
	workspaceapi.Pid, syscall.Signal,
) error {
	return nil
}

func (e *recordingDebugExecutor) Close() error {
	return nil
}
