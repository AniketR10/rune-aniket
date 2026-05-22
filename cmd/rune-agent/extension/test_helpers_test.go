// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.

package extension

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// testLocalFS is a minimal workspaceapi.FileSystem implementation that
// resolves relative paths against root and shells out to os.* for
// everything else. Used by e2e tests that need a real on-disk
// workspace for the agent's tools to walk.
type testLocalFS struct{ root string }

func (f testLocalFS) resolve(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(f.root, path)
}

func (f testLocalFS) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI("file://" + f.resolve(path))
}
func (f testLocalFS) OpenFile(path string, flag int, mode os.FileMode) (workspaceapi.File, error) {
	return os.OpenFile(f.resolve(path), flag, mode)
}
func (f testLocalFS) Remove(path string) error              { return os.Remove(f.resolve(path)) }
func (f testLocalFS) Stat(path string) (os.FileInfo, error) { return os.Stat(f.resolve(path)) }
func (f testLocalFS) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(f.resolve(name))
}
func (f testLocalFS) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(f.resolve(path), perm)
}

// testLocalExec is a minimal workspaceapi.Executor implementation that runs
// commands on the local machine. Used by gitidentity_test.go to invoke real
// git binaries against temp repos and by handler_test.go to invoke the
// bash tool against the host shell.
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
