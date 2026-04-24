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


package workspaceshell

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/processctx"
)

const testWidth = 200

// mockExecutor is a minimal workspaceapi.Executor for testing.
type mockExecutor struct {
	mu       sync.Mutex
	nextPid  workspaceapi.Pid
	starts   []workspaceapi.Cmd
	signals  []signalCall
	watchers map[workspaceapi.Pid]workspaceapi.ProcessWatcher
	startErr error
	// When true, SIGTERM also causes process exit in Signal.
	termExits bool
}

type signalCall struct {
	pid workspaceapi.Pid
	sig syscall.Signal
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		nextPid:  1,
		watchers: make(map[workspaceapi.Pid]workspaceapi.ProcessWatcher),
	}
}

func (m *mockExecutor) Start(
	_ context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.startErr != nil {
		return 0, m.startErr
	}
	pid := m.nextPid
	m.nextPid++
	m.starts = append(m.starts, cmd)
	m.watchers[pid] = cmd.Watcher
	return pid, nil
}

func (m *mockExecutor) Signal(
	pid workspaceapi.Pid, sig syscall.Signal,
) error {
	m.mu.Lock()
	m.signals = append(m.signals, signalCall{pid: pid, sig: sig})
	w := m.watchers[pid]
	termExits := m.termExits
	m.mu.Unlock()
	// Simulate SIGKILL causing immediate process exit.
	if sig == syscall.SIGKILL && w != nil {
		w.WatchProcess() <- nil
	}
	if sig == syscall.SIGTERM && termExits && w != nil {
		w.WatchProcess() <- nil
	}
	return nil
}

func (m *mockExecutor) Close() error { return nil }

// exitProcess simulates a process exiting successfully.
func (m *mockExecutor) exitProcess(pid workspaceapi.Pid) {
	m.exitProcessWithError(pid, nil)
}

// exitProcessWithError simulates a process exiting with
// the given error.
func (m *mockExecutor) exitProcessWithError(
	pid workspaceapi.Pid, err error,
) {
	m.mu.Lock()
	w := m.watchers[pid]
	m.mu.Unlock()
	if w != nil {
		w.WatchProcess() <- err
	}
}

func collectText(
	t *testing.T,
	iter iterator.Iterator[component.Responsive],
) []string {
	t.Helper()
	ctx := context.Background()
	var lines []string
	for {
		item, ok := iter.Next(ctx)
		if !ok {
			break
		}
		h := item.Height(testWidth)
		if h <= 0 {
			continue
		}
		w := term.NewStringWriter(testWidth, h)
		item.Resize(testWidth, h)
		item.Draw(w)
		_ = w.Flush()
		lines = append(lines, w.String())
	}
	require.NoError(t, iter.Err())
	return lines
}

func collectRenderedText(
	t *testing.T,
	iter iterator.Iterator[component.Responsive],
) string {
	t.Helper()
	return strings.Join(collectText(t, iter), "\n")
}

func TestStartTracksProcess(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	exec.now = fixedTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ctx := context.Background()
	pid, err := exec.Start(ctx, workspaceapi.Cmd{
		Path: "/usr/bin/gopls",
		Args: []string{"-rpc.trace"},
		Dir:  "/home/user",
	})
	require.NoError(t, err)
	assert.Equal(t, workspaceapi.Pid(1), pid)

	// Process should appear in status output.
	iter := exec.handleStatus()
	defer func() { _ = iter.Close() }()
	out := collectRenderedText(t, iter)
	assert.Contains(t, out, "PID")
	assert.Contains(t, out, "UPTIME")
	assert.Contains(t, out, "LAST ERR")
	assert.Contains(t, out, "COMMAND")
	assert.Contains(t, out, "gopls")
	assert.Contains(t, out, "-rpc.trace")
}

func fixedTime(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func TestStartPipesOriginalWatcher(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	origCh := make(chan error, 1)
	origWatcher := workspaceapi.ChanProcessWatcher(origCh)

	ctx := context.Background()
	pid, err := exec.Start(ctx, workspaceapi.Cmd{
		Path:    "/usr/bin/ls",
		Watcher: origWatcher,
	})
	require.NoError(t, err)

	// Simulate process exit.
	mock.exitProcess(pid)

	// Original watcher should also be notified.
	exitErr := <-origCh
	assert.NoError(t, exitErr)
}

func TestExitRemovesProcess(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	ctx := context.Background()
	pid, err := exec.Start(ctx, workspaceapi.Cmd{
		Path: "/usr/bin/ls",
	})
	require.NoError(t, err)

	// Simulate exit.
	mock.exitProcess(pid)

	// Wait for goroutine to clean up by checking ps.
	assert.Eventually(t, func() bool {
		exec.mu.RLock()
		defer exec.mu.RUnlock()
		return len(exec.processes) == 0
	}, 1e9, 1e6)
}

func TestStartError(t *testing.T) {
	mock := newMockExecutor()
	mock.startErr = errors.New("boom")
	exec := NewExecutor(mock)

	ctx := context.Background()
	_, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/x"})
	assert.Error(t, err)

	// No process tracked.
	exec.mu.RLock()
	assert.Empty(t, exec.processes)
	exec.mu.RUnlock()
}

func TestSignalDelegates(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	err := exec.Signal(42, syscall.SIGTERM)
	require.NoError(t, err)

	mock.mu.Lock()
	assert.Equal(t, []signalCall{{pid: 42, sig: syscall.SIGTERM}}, mock.signals)
	mock.mu.Unlock()
}

func TestHandleCommandStatus(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	exec.now = fixedTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ctx := context.Background()
	_, err := exec.Start(ctx, workspaceapi.Cmd{
		Path: "/usr/bin/gopls",
		Args: []string{"-rpc.trace"},
	})
	require.NoError(t, err)
	_, err = exec.Start(ctx, workspaceapi.Cmd{
		Path: "/usr/bin/bash",
	})
	require.NoError(t, err)

	iter, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"status"},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectRenderedText(t, iter)
	assert.Contains(t, out, "PID")
	assert.Contains(t, out, "UPTIME")
	assert.Contains(t, out, "LAST ERR")
	assert.Contains(t, out, "COMMAND")
	assert.Contains(t, out, "gopls")
	assert.Contains(t, out, "bash")
}

func TestDefaultSubcommandIsHelp(t *testing.T) {
	exec := NewExecutor(newMockExecutor())
	ctx := context.Background()

	iter, err := exec.HandleCommand(ctx, repl.Command{Name: "process"}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectRenderedText(t, iter)
	combined := ""
	combined += out
	assert.Contains(t, combined, "process status")
	assert.Contains(t, combined, "process audit")
	assert.Contains(t, combined, "process tree")
	assert.Contains(t, combined, "process info")
	assert.Contains(t, combined, "process signal")
	assert.Contains(t, combined, "process stop")
}

func TestHandleCommandSignal(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	ctx := context.Background()
	pid, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/sleep"})
	require.NoError(t, err)

	iter, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"signal", strconv.Itoa(int(pid))},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	mock.mu.Lock()
	require.Len(t, mock.signals, 1)
	assert.Equal(t, pid, mock.signals[0].pid)
	assert.Equal(t, syscall.SIGTERM, mock.signals[0].sig)
	mock.mu.Unlock()
}

func TestHandleCommandSignalWithFlagSignal(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	ctx := context.Background()
	pid, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/sleep"})
	require.NoError(t, err)

	iter, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"signal", "-9", strconv.Itoa(int(pid))},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	mock.mu.Lock()
	require.Len(t, mock.signals, 1)
	assert.Equal(t, pid, mock.signals[0].pid)
	assert.Equal(t, syscall.SIGKILL, mock.signals[0].sig)
	mock.mu.Unlock()
}

func TestHandleCommandSignalWithTrailingSignal(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	ctx := context.Background()
	pid, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/sleep"})
	require.NoError(t, err)

	iter, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"signal", strconv.Itoa(int(pid)), "9"},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	mock.mu.Lock()
	require.Len(t, mock.signals, 1)
	assert.Equal(t, pid, mock.signals[0].pid)
	assert.Equal(t, syscall.SIGKILL, mock.signals[0].sig)
	mock.mu.Unlock()
}

func TestHandleCommandSignalNoArgs(t *testing.T) {
	exec := NewExecutor(newMockExecutor())
	ctx := context.Background()
	_, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"signal"},
	}, repl.NopProgressWriter())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "usage")
}

func TestHandleCommandSignalInvalidPid(t *testing.T) {
	exec := NewExecutor(newMockExecutor())
	ctx := context.Background()
	_, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"signal", "abc"},
	}, repl.NopProgressWriter())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pid")
}

func TestHandleCommandSignalInvalidSignal(t *testing.T) {
	exec := NewExecutor(newMockExecutor())
	ctx := context.Background()
	_, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"signal", "-xyz", "1"},
	}, repl.NopProgressWriter())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signal")
}

func TestHandleCommandStop(t *testing.T) {
	mock := newMockExecutor()
	mock.termExits = true // SIGTERM causes exit
	exec := NewExecutor(mock)

	ctx := context.Background()
	pid, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/sleep"})
	require.NoError(t, err)

	iter, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"stop", strconv.Itoa(int(pid))},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectText(t, iter)
	assert.NotEmpty(t, out)
	assert.Contains(t, out[0], "stopped")

	mock.mu.Lock()
	// Graceful stop sends SIGTERM first.
	require.GreaterOrEqual(t, len(mock.signals), 1)
	assert.Equal(t, pid, mock.signals[0].pid)
	assert.Equal(t, syscall.SIGTERM, mock.signals[0].sig)
	mock.mu.Unlock()
}

func TestHandleCommandUnknown(t *testing.T) {
	exec := NewExecutor(newMockExecutor())
	ctx := context.Background()
	_, err := exec.HandleCommand(ctx, repl.Command{Name: "nope"}, repl.NopProgressWriter())
	assert.True(t, errors.Is(err, repl.ErrNotFound))
}

func TestHandleCommandUnknownSubcommand(t *testing.T) {
	exec := NewExecutor(newMockExecutor())
	ctx := context.Background()
	_, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"nope"},
	}, repl.NopProgressWriter())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown process subcommand")
}

func TestCompleteSignalPids(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	ctx := context.Background()
	// Start processes with PIDs 1, 2, 3.
	for range 3 {
		_, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/x"})
		require.NoError(t, err)
	}

	// Complete "process signal 1" → should return "1".
	iter, err := exec.Complete(ctx, "process", []string{"signal", "1"})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	got, err := iterator.ToSlice(ctx, iter)
	require.NoError(t, err)
	assert.Equal(t, []string{"1"}, got)

	// Complete "process signal " with empty prefix → all PIDs.
	iter2, err := exec.Complete(ctx, "process", []string{"signal", ""})
	require.NoError(t, err)
	defer func() { _ = iter2.Close() }()
	got2, err := iterator.ToSlice(ctx, iter2)
	require.NoError(t, err)
	assert.Equal(t, []string{"1", "2", "3"}, got2)
}

func TestCompleteStopPids(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	ctx := context.Background()
	_, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/x"})
	require.NoError(t, err)

	iter, err := exec.Complete(ctx, "process", []string{"stop", ""})
	require.NoError(t, err)
	got, err := iterator.ToSlice(ctx, iter)
	_ = iter.Close()
	require.NoError(t, err)
	assert.Equal(t, []string{"1"}, got)
}

func TestCompleteSubcommands(t *testing.T) {
	exec := NewExecutor(newMockExecutor())
	ctx := context.Background()

	iter, err := exec.Complete(ctx, "process", []string{})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	got, err := iterator.ToSlice(ctx, iter)
	require.NoError(t, err)
	assert.Equal(t, []string{"status", "audit", "tree", "info", "signal", "stop"}, got)

	iter2, err := exec.Complete(ctx, "process", []string{"st"})
	require.NoError(t, err)
	defer func() { _ = iter2.Close() }()
	got2, err := iterator.ToSlice(ctx, iter2)
	require.NoError(t, err)
	assert.Equal(t, []string{"status", "stop"}, got2)
}

func TestHelp(t *testing.T) {
	exec := NewExecutor(newMockExecutor())
	ctx := context.Background()
	iter, err := exec.Help(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectText(t, iter)
	assert.NotEmpty(t, out)
	combined := ""
	for _, line := range out {
		combined += line
	}
	assert.Contains(t, combined, "process status")
	assert.Contains(t, combined, "process audit")
	assert.Contains(t, combined, "process tree")
	assert.Contains(t, combined, "process info")
	assert.Contains(t, combined, "process signal")
	assert.Contains(t, combined, "process stop")
}

func TestPSSortedByPid(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	exec.now = fixedTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ctx := context.Background()
	// Start 3 processes.
	for _, path := range []string{"/c", "/a", "/b"} {
		_, err := exec.Start(ctx, workspaceapi.Cmd{Path: path})
		require.NoError(t, err)
	}

	iter := exec.handleStatus()
	defer func() { _ = iter.Close() }()
	out := collectRenderedText(t, iter)

	// PIDs should be 1, 2, 3 in order.
	idxC := strings.Index(out, "/c")
	idxA := strings.Index(out, "/a")
	idxB := strings.Index(out, "/b")
	require.NotEqual(t, -1, idxC)
	require.NotEqual(t, -1, idxA)
	require.NotEqual(t, -1, idxB)
	assert.Less(t, idxC, idxA)
	assert.Less(t, idxA, idxB)
}

func TestPSShowsUptime(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	exec.now = fixedTime(started)

	ctx := context.Background()
	_, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/x"})
	require.NoError(t, err)

	// Advance time by 5 minutes and 32 seconds.
	exec.now = fixedTime(started.Add(5*time.Minute + 32*time.Second))

	iter := exec.handleStatus()
	defer func() { _ = iter.Close() }()
	out := collectRenderedText(t, iter)
	assert.Contains(t, out, "5m32s")
}

func TestExitTracksStats(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	exec.now = fixedTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ctx := context.Background()
	cmd := workspaceapi.Cmd{Path: "/bin/x", Args: []string{"a"}}

	pid, err := exec.Start(ctx, cmd)
	require.NoError(t, err)

	// Simulate exit with error.
	mock.exitProcessWithError(pid, errors.New("segfault"))

	key := makeCmdKey(cmd.Path, cmd.Args)
	assert.Eventually(t, func() bool {
		exec.mu.RLock()
		defer exec.mu.RUnlock()
		return exec.stats[key] != nil
	}, 1e9, 1e6)

	exec.mu.RLock()
	assert.EqualError(t, exec.stats[key].lastErr, "segfault")
	exec.mu.RUnlock()
}

func TestStatusShowsLastErr(t *testing.T) {
	// lastErr is tracked per cmdKey and displayed for any running
	// process that shares that key.
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	exec.now = fixedTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ctx := context.Background()
	cmd := workspaceapi.Cmd{Path: "/bin/x"}

	pid, err := exec.Start(ctx, cmd)
	require.NoError(t, err)
	mock.exitProcessWithError(pid, errors.New("process failed very badly with a long error"))
	assert.Eventually(t, func() bool {
		exec.mu.RLock()
		defer exec.mu.RUnlock()
		return len(exec.processes) == 0
	}, 1e9, 1e6)

	_, err = exec.Start(ctx, cmd)
	require.NoError(t, err)

	iter := exec.handleStatus()
	defer func() { _ = iter.Close() }()
	out := collectRenderedText(t, iter)
	assert.Contains(t, out, "LAST ERR")
	assert.Contains(t, out, "process failed very badly")
	assert.Contains(t, out, "…")
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{45 * time.Second, "45s"},
		{5*time.Minute + 32*time.Second, "5m32s"},
		{2*time.Hour + 15*time.Minute, "2h15m"},
		{36 * time.Hour, "1d12h"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, formatDuration(tc.d), "duration=%v", tc.d)
	}
}

func TestContextParentPidTracksParent(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	ctx := context.Background()
	parent, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/parent"})
	require.NoError(t, err)

	childCtx := ContextWithParentPid(ctx, parent)
	child, err := exec.Start(childCtx, workspaceapi.Cmd{
		Path: "/bin/child",
	})
	require.NoError(t, err)

	exec.mu.RLock()
	parentInfo := exec.processes[parent]
	childInfo := exec.processes[child]
	exec.mu.RUnlock()

	assert.Equal(t, workspaceapi.Pid(0), parentInfo.parent)
	assert.Equal(t, parent, childInfo.parent)
}

func TestHandleCommandTree(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	exec.now = fixedTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ctx := context.Background()
	_, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/a"})
	require.NoError(t, err)
	_, err = exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/b"})
	require.NoError(t, err)

	iter, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"tree"},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectRenderedText(t, iter)
	assert.Contains(t, out, "Process tree")
	assert.Contains(t, out, "PID 1")
	assert.Contains(t, out, "PPID: —")
	assert.Contains(t, out, "/bin/a")
	assert.Contains(t, out, "/bin/b")
}

func TestHandleCommandTreeWithParent(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	exec.now = fixedTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ctx := context.Background()
	parent, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/ext"})
	require.NoError(t, err)
	childCtx := ContextWithParentPid(ctx, parent)
	_, err = exec.Start(childCtx, workspaceapi.Cmd{
		Path: "/bin/lsp",
	})
	require.NoError(t, err)

	iter, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"tree"},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectRenderedText(t, iter)
	assert.Contains(t, out, "/bin/ext")
	assert.Contains(t, out, "PPID: —")
	assert.Contains(t, out, "/bin/lsp")
	assert.Contains(t, out, "PPID: "+strconv.Itoa(int(parent)))
}

func TestHandleCommandInfo(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	started := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	exec.now = fixedTime(started)

	ctx := context.Background()
	pid, err := exec.Start(ctx, workspaceapi.Cmd{
		Path: "/usr/bin/gopls",
		Args: []string{"-rpc.trace"},
		Dir:  "/home/user/project",
		Env:  []string{"GOPATH=/go", "HOME=/home/user"},
	})
	require.NoError(t, err)

	// Advance time 10 minutes.
	exec.now = fixedTime(started.Add(10 * time.Minute))

	iter, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"info", strconv.Itoa(int(pid))},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectRenderedText(t, iter)
	combined := out + "\n"
	assert.Contains(t, combined, "PID:")
	assert.Contains(t, combined, "1")
	assert.Contains(t, combined, "Command:")
	assert.Contains(t, combined, "gopls -rpc.trace")
	assert.Contains(t, combined, "Directory:")
	assert.Contains(t, combined, "/home/user/project")
	assert.Contains(t, combined, "Started:")
	assert.Contains(t, combined, "Uptime:")
	assert.Contains(t, combined, "10m0s")
	assert.Contains(t, combined, "Environment:")
	assert.Contains(t, combined, "GOPATH=/go")
	assert.Contains(t, combined, "HOME=/home/user")
}

func TestHandleCommandInfoNotFound(t *testing.T) {
	exec := NewExecutor(newMockExecutor())
	ctx := context.Background()
	_, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"info", "999"},
	}, repl.NopProgressWriter())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestHandleCommandInfoNoArgs(t *testing.T) {
	exec := NewExecutor(newMockExecutor())
	ctx := context.Background()
	_, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"info"},
	}, repl.NopProgressWriter())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "usage")
}

func TestCompleteInfoPids(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	ctx := context.Background()
	_, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/x"})
	require.NoError(t, err)

	iter, err := exec.Complete(ctx, "process", []string{"info", ""})
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	got, err := iterator.ToSlice(ctx, iter)
	require.NoError(t, err)
	assert.Equal(t, []string{"1"}, got)
}

func TestProcessDoneChannelClosedOnExit(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	ctx := context.Background()
	pid, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/x"})
	require.NoError(t, err)

	// Grab done channel.
	exec.mu.RLock()
	done := exec.processes[pid].done
	exec.mu.RUnlock()

	// Simulate exit.
	mock.exitProcess(pid)

	// done should be closed.
	select {
	case <-done:
		// ok
	case <-time.After(time.Second):
		t.Fatal("done channel not closed after process exit")
	}
}

func TestHandleCommandStopGraceful(t *testing.T) {
	mock := newMockExecutor()
	mock.termExits = true // SIGTERM causes immediate exit
	exec := NewExecutor(mock)

	ctx := context.Background()
	pid, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/sleep"})
	require.NoError(t, err)

	iter, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"stop", strconv.Itoa(int(pid))},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectText(t, iter)
	require.NotEmpty(t, out)
	assert.Contains(t, out[0], "stopped")

	// Only SIGTERM should have been sent.
	mock.mu.Lock()
	require.Len(t, mock.signals, 1)
	assert.Equal(t, syscall.SIGTERM, mock.signals[0].sig)
	mock.mu.Unlock()
}

func TestHandleCommandStopForcedAfterTimeout(t *testing.T) {
	mock := newMockExecutor()
	// Don't set termExits — SIGTERM won't cause exit.
	exec := NewExecutor(mock)
	exec.stopGrace = 10 * time.Millisecond // Short grace for test.

	ctx := context.Background()
	pid, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/sleep"})
	require.NoError(t, err)

	iter, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"stop", strconv.Itoa(int(pid))},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectText(t, iter)
	require.NotEmpty(t, out)
	assert.Contains(t, out[0], "killed")
	assert.Contains(t, out[0], "timed out")

	// Both SIGTERM and SIGKILL should have been sent.
	mock.mu.Lock()
	require.Len(t, mock.signals, 2)
	assert.Equal(t, syscall.SIGTERM, mock.signals[0].sig)
	assert.Equal(t, syscall.SIGKILL, mock.signals[1].sig)
	mock.mu.Unlock()
}

func TestHandleCommandAuditShowsAllProcesses(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	started := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	exec.now = fixedTime(started)

	ctx := context.Background()

	// Start two processes.
	pid1, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/a"})
	require.NoError(t, err)
	_, err = exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/b"})
	require.NoError(t, err)

	// Exit the first process.
	exec.now = fixedTime(started.Add(5 * time.Second))
	mock.exitProcessWithError(pid1, errors.New("crash"))

	// Wait for exit goroutine to run.
	assert.Eventually(t, func() bool {
		exec.mu.RLock()
		defer exec.mu.RUnlock()
		_, running := exec.processes[pid1]
		return !running
	}, time.Second, time.Millisecond)

	// Advance time for uptime display.
	exec.now = fixedTime(started.Add(10 * time.Second))

	// Status should show only the running process.
	statusIter := exec.handleStatus()
	defer func() { _ = statusIter.Close() }()
	statusOut := collectRenderedText(t, statusIter)
	assert.Contains(t, statusOut, "/bin/b")

	// Audit should show both processes.
	auditIter := exec.handleAudit()
	defer func() { _ = auditIter.Close() }()
	auditOut := collectRenderedText(t, auditIter)
	assert.Contains(t, auditOut, "PID")
	assert.Contains(t, auditOut, "UPTIME")
	assert.Contains(t, auditOut, "LAST ERR")

	// Exited process should show its runtime (5s) and error.
	assert.Contains(t, auditOut, "/bin/a")
	assert.Contains(t, auditOut, "5s")
	assert.Contains(t, auditOut, "crash")

	// Running process should show current uptime (10s).
	assert.Contains(t, auditOut, "/bin/b")
	assert.Contains(t, auditOut, "10s")
}

func TestHandleCommandAuditViaDispatch(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	exec.now = fixedTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ctx := context.Background()
	_, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/x"})
	require.NoError(t, err)

	iter, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"audit"},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectRenderedText(t, iter)
	assert.Contains(t, out, "/bin/x")
}

func TestContextParentPidPropagation(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)

	ctx := context.Background()

	// Start parent process normally.
	parent, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/parent"})
	require.NoError(t, err)

	// Start child process with parent PID in context.
	childCtx := ContextWithParentPid(ctx, parent)
	child, err := exec.Start(childCtx, workspaceapi.Cmd{Path: "/bin/child"})
	require.NoError(t, err)

	exec.mu.RLock()
	parentInfo := exec.processes[parent]
	childInfo := exec.processes[child]
	exec.mu.RUnlock()

	assert.Equal(t, workspaceapi.Pid(0), parentInfo.parent,
		"parent should have no parent")
	assert.Equal(t, parent, childInfo.parent,
		"child should have parent PID from context")
}

func TestContextParentPidShowsInTree(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	exec.now = fixedTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ctx := context.Background()

	parent, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/ext"})
	require.NoError(t, err)

	childCtx := ContextWithParentPid(ctx, parent)
	_, err = exec.Start(childCtx, workspaceapi.Cmd{Path: "/bin/lsp"})
	require.NoError(t, err)

	iter, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"tree"},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectRenderedText(t, iter)
	assert.Contains(t, out, "/bin/ext")
	assert.Contains(t, out, "PPID: —")
	assert.Contains(t, out, "/bin/lsp")
	assert.Contains(t, out, strconv.Itoa(int(parent)))
}

func TestExtensionIDParentsSubprocesses(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	exec.now = fixedTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ctx := processctx.ContextWithExtensionID(context.Background(), "test-extension")
	parent, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/ext"})
	require.NoError(t, err)

	child, err := exec.Start(ctx, workspaceapi.Cmd{Path: "/bin/lsp"})
	require.NoError(t, err)

	exec.mu.RLock()
	parentInfo := exec.processes[parent]
	childInfo := exec.processes[child]
	exec.mu.RUnlock()

	assert.Equal(t, workspaceapi.Pid(0), parentInfo.parent)
	assert.Equal(t, parent, childInfo.parent)

	iter := exec.handleTree()
	defer func() { _ = iter.Close() }()
	out := collectRenderedText(t, iter)
	assert.Contains(t, out, "/bin/ext")
	assert.Contains(t, out, strconv.Itoa(int(parent)))
	assert.Contains(t, out, "/bin/lsp")
}

func TestHandleCommandInfoRedactsSecrets(t *testing.T) {
	mock := newMockExecutor()
	exec := NewExecutor(mock)
	exec.now = fixedTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))

	ctx := context.Background()
	pid, err := exec.Start(ctx, workspaceapi.Cmd{
		Path: "/bin/ext",
		Env: []string{
			"RUNE_CERT=secret-cert-value",
			"RUNE_TOKEN=secret-token-value",
			"IDE_CERT=ide-cert-value",
			"IDE_TOKEN=ide-token-value",
			"HOME=/home/user",
			"NO_EQUALS",
		},
	})
	require.NoError(t, err)

	iter, err := exec.HandleCommand(ctx, repl.Command{
		Name: "process",
		Args: []string{"info", strconv.Itoa(int(pid))},
	}, repl.NopProgressWriter())
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()

	out := collectRenderedText(t, iter)
	combined := out

	// Secret values must be redacted.
	assert.Contains(t, combined, "RUNE_CERT=****")
	assert.Contains(t, combined, "RUNE_TOKEN=****")
	assert.Contains(t, combined, "IDE_CERT=****")
	assert.Contains(t, combined, "IDE_TOKEN=****")

	// Original secret values must not appear.
	assert.NotContains(t, combined, "secret-cert-value")
	assert.NotContains(t, combined, "secret-token-value")
	assert.NotContains(t, combined, "ide-cert-value")
	assert.NotContains(t, combined, "ide-token-value")

	// Non-secret env vars must remain visible.
	assert.Contains(t, combined, "HOME=/home/user")
	assert.Contains(t, combined, "NO_EQUALS")
}

func TestRedactEnv(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"RUNE_CERT=abc", "RUNE_CERT=****"},
		{"RUNE_TOKEN=xyz", "RUNE_TOKEN=****"},
		{"IDE_CERT=123", "IDE_CERT=****"},
		{"IDE_TOKEN=456", "IDE_TOKEN=****"},
		{"HOME=/home/user", "HOME=/home/user"},
		{"PATH=/usr/bin", "PATH=/usr/bin"},
		{"NO_EQUALS", "NO_EQUALS"},
		{"RUNE_CERT=", "RUNE_CERT=****"},
		{"IDE_TOKEN=has=equals", "IDE_TOKEN=****"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, redactEnv(tc.input), "input=%q", tc.input)
	}
}

// schemeExecutorAdapter wraps a schemeapi.Executor as a
// workspaceapi.Executor for use with NewExecutor.
type schemeExecutorAdapter struct {
	s schemeapi.Executor
}

func (a schemeExecutorAdapter) Start(
	ctx context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	return a.s.StartCommand(ctx, cmd)
}

func (a schemeExecutorAdapter) Signal(
	pid workspaceapi.Pid, sig syscall.Signal,
) error {
	return a.s.Signal(pid, sig)
}

func (a schemeExecutorAdapter) Close() error {
	return a.s.Close()
}

func TestE2EProcessTreeWithRealExecutor(t *testing.T) {
	tmpDir := t.TempDir()

	uri, err := workspaceapi.ParseURI("file://" + tmpDir)
	require.NoError(t, err)

	scheme, err := workspace.NewFileScheme(
		context.Background(), config.NopConfig(), uri,
	)
	require.NoError(t, err)
	defer func() { _ = scheme.Close() }()

	exec := NewExecutor(schemeExecutorAdapter{s: scheme.(schemeapi.Executor)})

	ctx := context.Background()

	// Start a long-running "parent" process.
	parentPid, err := exec.Start(ctx, workspaceapi.Cmd{
		Path: "/bin/sleep",
		Args: []string{"30"},
	})
	require.NoError(t, err)
	defer func() { _ = exec.Signal(parentPid, syscall.SIGKILL) }()

	// Start a "child" process with logical parent context.
	childCtx := ContextWithParentPid(ctx, parentPid)
	childPid, err := exec.Start(childCtx, workspaceapi.Cmd{
		Path: "/bin/sleep",
		Args: []string{"30"},
	})
	require.NoError(t, err)
	defer func() { _ = exec.Signal(childPid, syscall.SIGKILL) }()

	// Verify the tree output shows the parent-child relationship.
	iter := exec.handleTree()
	defer func() { _ = iter.Close() }()
	out := collectRenderedText(t, iter)

	assert.Contains(t, out, "Process tree")
	assert.Contains(t, out, strconv.Itoa(int(parentPid)))
	assert.Contains(t, out, "PPID: —")
	assert.Contains(t, out, "/bin/sleep")
	assert.Contains(t, out, strconv.Itoa(int(childPid)))
	assert.Contains(t, out, "PPID: "+strconv.Itoa(int(parentPid)))
}
