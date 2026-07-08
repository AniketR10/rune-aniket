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

package sandbox

import (
	"context"
	"io"
	"sync"
	"syscall"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/debug"
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
