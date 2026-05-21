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
	"net"
	"testing"
	"time"

	"github.com/google/go-dap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/debugapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// fakeSubscriber is a no-op EventSubscriber for tests that do
// not exercise the event stream.
type fakeSubscriber struct{}

func (fakeSubscriber) OnEvent(dap.EventMessage) {}
func (fakeSubscriber) OnClose(string)           {}

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	uri, err := workspaceapi.ParseURI("file:///tmp/test")
	require.NoError(t, err)
	return New(uri, nil, nil, Config{})
}

// TestSessionIDRequired verifies that every method returns
// ErrSessionNotFound when called with an unknown session ID.
func TestSessionIDRequired(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	t.Cleanup(func() { _ = m.Close() })

	ctx := context.Background()
	_, err := m.Threads(ctx, "missing")
	assert.ErrorIs(t, err, debugapi.ErrSessionNotFound)

	_, err = m.Continue(ctx, "missing", &dap.ContinueArguments{})
	assert.ErrorIs(t, err, debugapi.ErrSessionNotFound)

	err = m.Terminate(ctx, "missing", &dap.TerminateArguments{})
	assert.ErrorIs(t, err, debugapi.ErrSessionNotFound)
}

// TestCreateSessionNoAdapter verifies CreateSession errors with
// ErrNoAdapterConfigured when the requested langID is not in
// Config.Adapters.
func TestCreateSessionNoAdapter(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)
	t.Cleanup(func() { _ = m.Close() })

	_, _, err := m.CreateSession(context.Background(), "unknown",
		debugapi.ClientCapabilities{}, fakeSubscriber{})
	assert.True(t, errors.Is(err, debugapi.ErrNoAdapterConfigured),
		"expected ErrNoAdapterConfigured, got %v", err)
}

// TestManagerCloseTerminatesAdapterAndGoroutines locks in the RUNE-180
// contract that Manager.Close cancels the manager ctx, cancels each
// session ctx (so exec.CommandContext kills the adapter subprocess),
// and waits for watchSession goroutines to exit.
func TestManagerCloseTerminatesAdapterAndGoroutines(t *testing.T) {
	t.Parallel()
	m := newTestManager(t)

	// Inject a session mirroring startSession's invariants instead of
	// launching a real DAP adapter (which would need a fake binary):
	// a *debugServer parented on m.ctx with an open conn, a watcher
	// channel, and a watchSession goroutine tracked by m.wg.
	a, b := net.Pipe()
	t.Cleanup(func() { _ = a.Close(); _ = b.Close() })

	watcher := make(chan error, 1)
	srv := newDebugServer(m.ctx, debugConfig{langID: "test", command: "test"},
		"/bin/test", nil, m.rootURI,
		debugapi.ClientCapabilities{}, fakeSubscriber{})
	srv.conn = a
	srv.watcher = watcher
	srv.alive = true

	const sessionID = "test-session"
	m.mu.Lock()
	m.sessions[sessionID] = srv
	m.mu.Unlock()

	m.wg.Add(1)
	done := make(chan struct{})
	go func() {
		defer m.wg.Done()
		defer close(done)
		m.watchSession(sessionID, &srv.cfg, srv)
	}()

	select {
	case <-done:
		t.Fatal("watchSession exited before Close")
	case <-time.After(50 * time.Millisecond):
	}

	require.NoError(t, m.Close())

	require.Error(t, m.ctx.Err(),
		"Manager.Close must cancel m.ctx")
	require.Error(t, srv.ctx.Err(),
		"Manager.Close must cancel each debugServer ctx so the "+
			"adapter subprocess is killed")
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watchSession goroutine did not exit after Manager.Close")
	}
}
