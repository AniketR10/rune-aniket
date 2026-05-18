// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.

package extension

import (
	"context"
	"os"
	"os/exec"
	"syscall"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// testLocalExec is a minimal workspaceapi.Executor implementation that runs
// commands on the local machine. Used by gitidentity_test.go to invoke real
// git binaries against temp repos.
type testLocalExec struct{}

func (testLocalExec) Start(ctx context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
	c := exec.CommandContext(ctx, cmd.Path, cmd.Args...)
	c.Dir = cmd.Dir
	c.Stdin = cmd.Stdin
	c.Stdout = cmd.Stdout
	c.Stderr = cmd.Stderr
	c.Env = cmd.Env

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

func (testLocalExec) Signal(pid workspaceapi.Pid, sig syscall.Signal) error {
	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return err
	}
	return proc.Signal(sig)
}

func (testLocalExec) Close() error { return nil }
