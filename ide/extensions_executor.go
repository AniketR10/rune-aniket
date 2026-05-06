// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY.

package ide

import (
	"context"
	"fmt"
	"syscall"

	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/ide/ideshell/workspaceshell"
	"unstable.build/go-tui/workspace"
)

// extensionsExecutor combines a local file scheme rooted at
// extensionsCmdDir with a workspaceshell.Executor that tracks
// extension binaries as their own root processes. It forces
// Cmd.Dir to "/tmp" for every StartCommand call.
//
// Extensions are user-owned local processes; the workspace
// directory has no special meaning to them and may not even exist
// on the host where the extension actually runs (e.g. an SSH
// workspace's path lives on the remote host, while the extension
// binary lives on the IDE host). Pinning Cmd.Dir to a path that is
// guaranteed to exist on every supported host keeps fork/exec
// deterministic regardless of the underlying scheme.
//
// Wrapping the local scheme in a workspaceshell.Executor lets the
// "extensions-process" REPL command list extension PIDs even
// though they bypass the workspace's own process executor.
type extensionsExecutor struct {
	shell *workspaceshell.Executor
}

// extensionsCmdDir is the working directory installed on every
// extension Cmd. /tmp is the only path POSIX guarantees to exist and
// to be writable by all users; we don't care where the extension
// runs as long as it doesn't blow up on chdir.
const extensionsCmdDir = "/tmp"

// newExtensionsExecutor builds a fresh extensionsExecutor backed by
// a local fileScheme rooted at extensionsCmdDir and a tracking
// workspaceshell.Executor.
func newExtensionsExecutor() (*extensionsExecutor, error) {
	rootURI, err := workspaceapi.CurrentUserHostURI(extensionsCmdDir)
	if err != nil {
		return nil, fmt.Errorf("extensions root uri: %w", err)
	}
	root, err := workspace.NewFileScheme(
		context.Background(), config.NopConfig(), rootURI)
	if err != nil {
		return nil, fmt.Errorf("new extensions root scheme: %w", err)
	}
	return newExtensionsExecutorFromUnderlying(
		workspaceExecutorAdapter{e: root}), nil
}

// newExtensionsExecutorFromUnderlying wraps the given underlying
// executor in a tracking workspaceshell.Executor. Used by tests so
// they can inject a recording mock instead of a real fileScheme.
func newExtensionsExecutorFromUnderlying(
	underlying workspaceapi.Executor,
) *extensionsExecutor {
	return &extensionsExecutor{
		shell: workspaceshell.NewExecutor(underlying),
	}
}

// StartCommand satisfies schemeapi.Executor. It rewrites Cmd.Dir to
// extensionsCmdDir and forwards to the tracking shell executor. Any
// caller-provided Dir is dropped on purpose: extensions don't get to
// pick their working directory.
func (e *extensionsExecutor) StartCommand(
	ctx context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	cmd.Dir = extensionsCmdDir
	return e.shell.StartCommand(ctx, cmd)
}

// Signal satisfies schemeapi.Executor.
func (e *extensionsExecutor) Signal(pid workspaceapi.Pid, sig syscall.Signal) error {
	return e.shell.Signal(pid, sig)
}

// Close satisfies schemeapi.Executor (and io.Closer).
func (e *extensionsExecutor) Close() error {
	return e.shell.Close()
}
