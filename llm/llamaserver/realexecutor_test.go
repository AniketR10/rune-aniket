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

package llamaserver_test

import (
	"context"
	"os/exec"
	"sync"
	"syscall"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/debug"
)

// realExecutor is a minimal os/exec-based schemeapi.Executor for the e2e
// suite. It launches real child processes (llama-server), kills them when
// the lifecycle context is cancelled, and drives each Cmd's Watcher on
// exit — matching the contract the pool relies on in production.
type realExecutor struct {
	mu       sync.Mutex
	next     workspaceapi.Pid
	procs    map[workspaceapi.Pid]*exec.Cmd
	wg       sync.WaitGroup
}

func (e *realExecutor) StartCommand(
	ctx context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	c := exec.Command(cmd.Path, cmd.Args...) // #nosec G204 -- test-only, args from our config
	c.Dir = cmd.Dir
	c.Env = cmd.Env
	c.Stdin = cmd.Stdin
	c.Stdout = cmd.Stdout
	c.Stderr = cmd.Stderr
	// New process group so we can signal the whole tree on cancel.
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := c.Start(); err != nil {
		return 0, err
	}

	e.mu.Lock()
	if e.procs == nil {
		e.procs = make(map[workspaceapi.Pid]*exec.Cmd)
	}
	e.next++
	pid := e.next
	e.procs[pid] = c
	e.mu.Unlock()

	var watchCh chan error
	if cmd.Watcher != nil {
		watchCh = cmd.Watcher.WatchProcess()
	}

	e.wg.Add(1)
	go debug.CapturePanicReport(func() {
		defer e.wg.Done()
		waitErr := make(chan error, 1)
		go debug.CapturePanicReport(func() { waitErr <- c.Wait() })

		select {
		case <-ctx.Done():
			// Kill the whole process group; the child may spawn helpers.
			if c.Process != nil {
				_ = syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
			}
			<-waitErr
		case err := <-waitErr:
			if watchCh != nil {
				select {
				case watchCh <- err:
				default:
				}
			}
		}
		e.mu.Lock()
		delete(e.procs, pid)
		e.mu.Unlock()
	})

	return pid, nil
}

func (e *realExecutor) Signal(pid workspaceapi.Pid, sig syscall.Signal) error {
	e.mu.Lock()
	c := e.procs[pid]
	e.mu.Unlock()
	if c == nil || c.Process == nil {
		return nil
	}
	return c.Process.Signal(sig)
}

func (e *realExecutor) Close() error { return nil }

// wait blocks until every launched process goroutine has exited. Used as a
// test cleanup to avoid leaking child processes across tests.
func (e *realExecutor) wait() {
	e.mu.Lock()
	for _, c := range e.procs {
		if c.Process != nil {
			_ = syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
		}
	}
	e.mu.Unlock()
	e.wg.Wait()
}
