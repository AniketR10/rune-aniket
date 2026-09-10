// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package sandbox

import (
	"context"
	"io"
	"sync"
	"syscall"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/internal/debug"
)

// teeExecutor wraps the executor that launches the extension binary.
// It tees the extension's stderr into the sandbox's own sink (log
// file and, in verbose mode, the sandbox stderr), records the pid and
// spawn time, and observes the process exit without disturbing the
// runner's own watcher.
type teeExecutor struct {
	schemeapi.Executor
	stderr io.Writer

	mu      sync.Mutex
	pid     workspaceapi.Pid
	spawned time.Time
	exitErr error
	exited  bool
	// exitCh is closed once the extension process exits.
	exitCh chan struct{}
}

func newTeeExecutor(inner schemeapi.Executor, stderr io.Writer) *teeExecutor {
	return &teeExecutor{
		Executor: inner,
		stderr:   stderr,
		exitCh:   make(chan struct{}),
	}
}

// StartCommand satisfies schemeapi.Executor.
func (t *teeExecutor) StartCommand(
	ctx context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	if t.stderr != nil {
		if cmd.Stderr != nil {
			cmd.Stderr = io.MultiWriter(cmd.Stderr, t.stderr)
		} else {
			cmd.Stderr = t.stderr
		}
	}
	// The host's stdin protocol reader blocks until the handshake
	// completes, which keeps exec.Cmd.Wait (and therefore exit
	// detection) stuck when the extension dies before handshaking.
	// Wrap stdin so it returns EOF once the process is gone.
	gone := make(chan struct{})
	if cmd.Stdin != nil {
		cmd.Stdin = &cancelableReader{r: cmd.Stdin, done: gone}
	}
	orig := cmd.Watcher
	ours := make(chan error, 1)
	cmd.Watcher = workspaceapi.ChanProcessWatcher(ours)
	go debug.CapturePanicReport(func() {
		err := <-ours
		t.mu.Lock()
		t.exitErr = err
		t.exited = true
		t.mu.Unlock()
		close(t.exitCh)
		if orig != nil && orig.WatchProcess() != nil {
			orig.WatchProcess() <- err
		}
	})

	pid, err := t.Executor.StartCommand(ctx, cmd)
	if err != nil {
		close(gone)
		return pid, err
	}
	t.mu.Lock()
	t.pid = pid
	t.spawned = time.Now()
	t.mu.Unlock()
	go debug.CapturePanicReport(func() {
		pollProcessGone(ctx, int(pid), gone)
	})
	return pid, nil
}

// pollProcessGone closes gone once pid no longer exists. exec.Cmd.Wait
// reaps the process before waiting for its io copiers, so signal 0
// starts failing with ESRCH shortly after the process exits even
// while Wait is still blocked.
func pollProcessGone(ctx context.Context, pid int, gone chan struct{}) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := syscall.Kill(pid, 0); err != nil {
				close(gone)
				return
			}
		case <-ctx.Done():
			close(gone)
			return
		}
	}
}

type readResult struct {
	data []byte
	err  error
}

// cancelableReader forwards Reads to r but fails with EOF once done
// is closed, abandoning any in-flight blocking Read. Read is called
// from a single goroutine (the exec stdin copier).
type cancelableReader struct {
	r    io.Reader
	done <-chan struct{}
	res  chan readResult
	busy bool
}

func (c *cancelableReader) Read(p []byte) (int, error) {
	select {
	case <-c.done:
		return 0, io.EOF
	default:
	}
	if c.res == nil {
		c.res = make(chan readResult, 1)
	}
	if !c.busy {
		c.busy = true
		buf := make([]byte, len(p))
		go debug.CapturePanicReport(func() {
			n, err := c.r.Read(buf)
			c.res <- readResult{data: buf[:n], err: err}
		})
	}
	select {
	case res := <-c.res:
		c.busy = false
		n := copy(p, res.data)
		return n, res.err
	case <-c.done:
		return 0, io.EOF
	}
}

func (t *teeExecutor) processPid() (workspaceapi.Pid, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.pid, t.pid != 0
}

// exitStatus reports whether the process has exited and its exit
// error (nil for a clean exit).
func (t *teeExecutor) exitStatus() (error, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.exitErr, t.exited
}

// waitExit blocks until the process exits or the timeout expires,
// reporting whether it exited.
func (t *teeExecutor) waitExit(timeout time.Duration) bool {
	select {
	case <-t.exitCh:
		return true
	case <-time.After(timeout):
		return false
	}
}
