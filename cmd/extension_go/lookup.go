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

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// goplsResolutionTimeout caps the total time spent probing the workspace
// host for a gopls binary so a hung remote shell cannot stall
// workspace bring-up.
const goplsResolutionTimeout = 3 * time.Second

// wellKnownGoplsPaths lists the paths the resolver Stat's when the
// shell probe fails. Paths beginning with "~/" are resolved against
// $HOME on the workspace host (skipped if $HOME cannot be read).
var wellKnownGoplsPaths = []string{
	"~/.rune/bin/gopls",
	"~/go/bin/gopls",
	"/usr/local/go/bin/gopls",
	"/opt/homebrew/bin/gopls",
	"/usr/local/bin/gopls",
}

// hasGoProjectFiles reports whether the workspace top-level directory
// contains a go.mod, go.sum, or go.work file. The check uses relative
// paths so the workspace's FileSystem resolves them against the
// workspace root.
func hasGoProjectFiles(_ context.Context, fs workspaceapi.FileSystem) bool {
	for _, name := range []string{"go.mod", "go.sum", "go.work"} {
		if _, err := fs.Stat(name); err == nil {
			return true
		}
	}
	return false
}

// resolveGoplsBinary attempts to locate the gopls binary on the
// workspace host. The result is either an absolute path to the binary
// or an error if every strategy failed; the caller is responsible for
// surfacing the failure to the user.
//
// Resolution order:
//  1. lspPath verbatim, when non-empty (config override wins).
//  2. `$SHELL -lc 'command -v gopls'` through the workspace executor.
//  3. fs.Stat against wellKnownGoplsPaths in order.
func resolveGoplsBinary(
	ctx context.Context,
	fs workspaceapi.FileSystem,
	exec workspaceapi.Executor,
	lspPath string,
) (string, error) {
	if lspPath != "" {
		return lspPath, nil
	}

	ctx, cancel := context.WithTimeout(ctx, goplsResolutionTimeout)
	defer cancel()

	shell, home := probeEnv(ctx, exec)

	if bin, err := probeShellLookup(ctx, exec, shell); err == nil {
		return bin, nil
	}

	if bin, ok := probeWellKnown(fs, home); ok {
		return bin, nil
	}

	return "", errors.New("gopls binary not found on workspace host")
}

// probeEnv reads $SHELL and $HOME from the workspace host by running
// `sh -c 'printf ...'`. Either value may be empty if the probe fails
// or the variable is unset.
func probeEnv(
	ctx context.Context, exec workspaceapi.Executor,
) (shell, home string) {
	var stdout bytes.Buffer
	ch := make(chan error, 1)
	cmd := workspaceapi.Cmd{
		Path:    "sh",
		Args:    []string{"-c", `printf '%s\n%s\n' "$SHELL" "$HOME"`},
		Stdout:  &stdout,
		Watcher: workspaceapi.ChanProcessWatcher(ch),
	}
	if _, err := exec.Start(ctx, cmd); err != nil {
		return "", ""
	}
	select {
	case err := <-ch:
		if err != nil {
			return "", ""
		}
	case <-ctx.Done():
		return "", ""
	}
	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	if len(lines) > 0 {
		shell = strings.TrimSpace(lines[0])
	}
	if len(lines) > 1 {
		home = strings.TrimSpace(lines[1])
	}
	return shell, home
}

// probeShellLookup runs `<shell> -lc 'command -v gopls'` and returns
// the absolute path printed by the shell, or an error if the probe
// failed or returned a non-absolute path. If shell is empty, "sh" is
// used.
func probeShellLookup(
	ctx context.Context, exec workspaceapi.Executor, shell string,
) (string, error) {
	if shell == "" {
		shell = "sh"
	}
	var stdout, stderr bytes.Buffer
	ch := make(chan error, 1)
	cmd := workspaceapi.Cmd{
		Path:    shell,
		Args:    []string{"-lc", "command -v gopls"},
		Stdout:  &stdout,
		Stderr:  &stderr,
		Watcher: workspaceapi.ChanProcessWatcher(ch),
	}
	if _, err := exec.Start(ctx, cmd); err != nil {
		return "", fmt.Errorf("start %s: %w", shell, err)
	}
	select {
	case err := <-ch:
		if err != nil {
			return "", err
		}
	case <-ctx.Done():
		return "", ctx.Err()
	}
	out := strings.TrimSpace(stdout.String())
	if out == "" || !strings.HasPrefix(out, "/") {
		return "", fmt.Errorf("shell probe produced no absolute path: %q", out)
	}
	// command -v may print multiple lines for aliases/functions; take
	// the first absolute path it printed.
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "/") {
			return line, nil
		}
	}
	return "", fmt.Errorf("shell probe produced no absolute path: %q", out)
}

// probeWellKnown Stat's each well-known location in order, returning
// the first path that exists and is not a directory. Tilde-prefixed
// paths are skipped when home is empty.
func probeWellKnown(
	fs workspaceapi.FileSystem, home string,
) (string, bool) {
	for _, candidate := range wellKnownGoplsPaths {
		full := candidate
		if strings.HasPrefix(candidate, "~/") {
			if home == "" {
				continue
			}
			full = path.Join(home, strings.TrimPrefix(candidate, "~/"))
		}
		info, err := fs.Stat(full)
		if err != nil || info == nil || info.IsDir() {
			continue
		}
		return full, true
	}
	return "", false
}
