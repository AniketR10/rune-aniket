// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.

package hooks

import (
	"context"
	"os"
	"os/exec"
	"syscall"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// localExec is a test-only Executor that shells out via os/exec. It
// only exists to give the hooks package an end-to-end exec path
// without depending on a workspace.
type localExec struct{}

func (localExec) Start(ctx context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
	c := exec.CommandContext(ctx, cmd.Path, cmd.Args...)
	c.Dir = cmd.Dir
	c.Stdin = cmd.Stdin
	c.Stdout = cmd.Stdout
	c.Stderr = cmd.Stderr
	c.Env = append(os.Environ(), cmd.Env...)
	if err := c.Start(); err != nil {
		return 0, err
	}
	pid := workspaceapi.Pid(c.Process.Pid)
	go func() {
		err := c.Wait()
		if cmd.Watcher != nil {
			cmd.Watcher.WatchProcess() <- err
		}
	}()
	return pid, nil
}

func (localExec) Signal(pid workspaceapi.Pid, sig syscall.Signal) error {
	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return err
	}
	return proc.Signal(sig)
}

func (localExec) Close() error { return nil }
