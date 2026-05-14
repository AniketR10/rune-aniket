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
// workspace bring-up. Only the shell probe consumes meaningful time;
// the DataDir and well-known checks are local Stat calls.
const goplsResolutionTimeout = 5 * time.Second

// wellKnownGoplsPaths lists the paths the resolver Stats after the
// per-workspace `<DataDir>/bin/gopls` check fails. Tilde-prefixed
// paths are passed verbatim to fs.Stat, which routes them through
// workspaceapi.ExpandPath so "~/" expands against the workspace
// host's $HOME without any extra shell round-trip.
var wellKnownGoplsPaths = []string{
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
// Resolution order (cheapest, most deterministic first):
//  1. <dataDir>/bin/gopls — the standard package-install location.
//     Rune installs gopls there, so this is the answer for the vast
//     majority of workspaces.
//  2. fs.Stat against wellKnownGoplsPaths in order. Tilde paths are
//     expanded by fs.Stat via workspaceapi.ExpandPath against the
//     workspace host's $HOME.
//  3. `sh -lc 'command -v gopls'` through the workspace executor, as
//     a last resort for users who installed gopls outside the
//     well-known set.
//
// The `lsp_path` config override is handled by the caller in
// resolveGoplsForWorkspace and bypasses this function entirely.
func resolveGoplsBinary(
	ctx context.Context,
	fs workspaceapi.FileSystem,
	exec workspaceapi.Executor,
	dataDir string,
) (string, error) {
	candidate := path.Join(dataDir, "bin", "gopls")
	if info, err := fs.Stat(candidate); err == nil && info != nil && !info.IsDir() {
		return candidate, nil
	}

	if bin, ok := probeWellKnown(fs); ok {
		return bin, nil
	}

	ctx, cancel := context.WithTimeout(ctx, goplsResolutionTimeout)
	defer cancel()
	if bin, err := probeShellLookup(ctx, exec); err == nil {
		return bin, nil
	}

	return "", errors.New("gopls binary not found on workspace host")
}

// probeShellLookup runs `sh -lc 'command -v gopls'` through the
// workspace executor and returns the absolute path printed by the
// shell, or an error if the probe failed or returned a non-absolute
// path. The login flag is preserved so users whose gopls lives only
// on the interactive PATH still get resolved.
func probeShellLookup(
	ctx context.Context, exec workspaceapi.Executor,
) (string, error) {
	var stdout, stderr bytes.Buffer
	ch := make(chan error, 1)
	cmd := workspaceapi.Cmd{
		Path:    "sh",
		Args:    []string{"-lc", "command -v gopls"},
		Stdout:  &stdout,
		Stderr:  &stderr,
		Watcher: workspaceapi.ChanProcessWatcher(ch),
	}
	if _, err := exec.Start(ctx, cmd); err != nil {
		return "", fmt.Errorf("start sh: %w", err)
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

// probeWellKnown Stats each well-known location in order and returns
// the first path that exists and is not a directory. Tilde-prefixed
// candidates are expanded via fs.URI (which calls
// workspaceapi.ExpandPath against the workspace host's $HOME) before
// being returned, so the resolver always hands a fully-resolved
// absolute path to gopls.
func probeWellKnown(fs workspaceapi.FileSystem) (string, bool) {
	for _, candidate := range wellKnownGoplsPaths {
		uri, err := fs.URI(candidate)
		if err != nil {
			continue
		}
		full := uri.Path()
		info, err := fs.Stat(full)
		if err != nil || info == nil || info.IsDir() {
			continue
		}
		return full, true
	}
	return "", false
}
