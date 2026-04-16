// Copyright 2026 Unstable Build, LLC.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the
// Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// See <https://www.gnu.org/licenses/> for a copy of the license.

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
